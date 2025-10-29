package systemd

import (
	"context"
	"os"
	"sync"
	"testing"
)

func TestParallelConnection(t *testing.T) {
	if !IsRunningSystemd() {
		t.Skip("Test requires systemd.")
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
		doneWg.Add(1)
		go func(dm *dbusConnManager) {
			defer doneWg.Done()
			select {
			case <-ctx.Done():
				return
			case <-startCh:
				_, err := dm.newConnection()
				if err != nil {
					// Only bother trying to send the first error.
					select {
					case errCh <- err:
					default:
					}
					cancel()
				}
			}
		}(dm)
	}
	close(startCh) // trigger all connection attempts
	doneWg.Wait()

	select {
	case err := <-errCh:
		t.Fatal(err)
	default:
	}
}
