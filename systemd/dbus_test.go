package systemd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"

	systemdDbus "github.com/coreos/go-systemd/v22/dbus"
	dbus "github.com/godbus/dbus/v5"
	"golang.org/x/sys/unix"
)

func TestParallelConnection(t *testing.T) {
	if !IsRunningSystemd() {
		t.Skip("Test requires systemd.")
	}
	if _, ok := os.LookupEnv("CGROUPS_ALLOW_UNSAFE_TESTS"); !ok {
		t.Skip("skipping unsafe test (can kill your desktop session); " +
			"set CGROUPS_ALLOW_UNSAFE_TESTS=true to enable")
	}
	var dms []*dbusConnManager
	for range 600 {
		dms = append(dms, newDbusConnManager(os.Geteuid() != 0))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var (
		doneWg  sync.WaitGroup
		startCh = make(chan struct{})
		errCh   = make(chan error, 1)
	)
	for _, dm := range dms {
		doneWg.Go(func() {
			select {
			case <-ctx.Done():
				return
			case <-startCh:
				conn, err := dm.newConnection()
				if err != nil {
					// Only bother trying to send the first error.
					select {
					case errCh <- err:
					default:
					}
					cancel()
					return
				}
				// Release the slot so that refused connections can get in on retry.
				conn.Close()
			}
		})
	}
	close(startCh) // trigger all connection attempts
	doneWg.Wait()

	select {
	case err := <-errCh:
		t.Fatal(err)
	default:
	}
}

func TestNewConnection(t *testing.T) {
	if os.Geteuid() != 0 {
		// newConnection only falls back as root, and newDbusConnManager(false)
		// would panic after the rootless managers non-root tests create.
		t.Skip("Test requires root.")
	}
	origSystem, origPrivate := newSystemBusConn, newPrivateSocketConn
	defer func() {
		newSystemBusConn = origSystem
		newPrivateSocketConn = origPrivate
	}()
	dm := newDbusConnManager(false)

	noBus := errors.New("no system bus")
	limitsExceeded := dbus.Error{Name: "org.freedesktop.DBus.Error.LimitsExceeded"}

	// Each constructor returns the listed errors in order, then succeeds.
	for _, tc := range []struct {
		name             string
		systemErrs       []error
		privateErrs      []error
		wantSystemCalls  int
		wantPrivateCalls int
	}{
		{
			name:            "LimitsExceeded is retried",
			systemErrs:      []error{limitsExceeded, limitsExceeded},
			wantSystemCalls: 3,
		},
		{
			name:            "EAGAIN from the system bus is retried",
			systemErrs:      []error{fmt.Errorf("dial unix /run/dbus/system_bus_socket: %w", unix.EAGAIN)},
			wantSystemCalls: 2,
		},
		{
			name:             "other errors fall back",
			systemErrs:       []error{noBus},
			wantSystemCalls:  1,
			wantPrivateCalls: 1,
		},
		{
			name:             "EAGAIN from the private socket is retried",
			systemErrs:       []error{noBus, noBus},
			privateErrs:      []error{fmt.Errorf("dial unix /run/systemd/private: %w", unix.EAGAIN)},
			wantSystemCalls:  2,
			wantPrivateCalls: 2,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			systemCalls, privateCalls := 0, 0
			wantConn := &systemdDbus.Conn{}
			newSystemBusConn = func(context.Context) (*systemdDbus.Conn, error) {
				systemCalls++
				if systemCalls <= len(tc.systemErrs) {
					return nil, tc.systemErrs[systemCalls-1]
				}
				return wantConn, nil
			}
			newPrivateSocketConn = func(context.Context) (*systemdDbus.Conn, error) {
				privateCalls++
				if privateCalls <= len(tc.privateErrs) {
					return nil, tc.privateErrs[privateCalls-1]
				}
				return wantConn, nil
			}

			conn, err := dm.newConnection()
			if err != nil {
				t.Fatal(err)
			}
			if conn != wantConn {
				t.Error("newConnection did not return the successful connection")
			}
			if systemCalls != tc.wantSystemCalls || privateCalls != tc.wantPrivateCalls {
				t.Errorf("got %d system bus and %d private socket calls, want %d and %d",
					systemCalls, privateCalls, tc.wantSystemCalls, tc.wantPrivateCalls)
			}
		})
	}
}
