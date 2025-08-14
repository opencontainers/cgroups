package systemd

import (
	"os"
	"reflect"
	"testing"

	systemdDbus "github.com/coreos/go-systemd/v22/dbus"
	"github.com/opencontainers/cgroups"
)

func newManager(t *testing.T, config *cgroups.Cgroup) (m cgroups.Manager) {
	t.Helper()
	var err error

	if cgroups.IsCgroup2UnifiedMode() {
		m, err = NewUnifiedManager(config, "")
	} else {
		m, err = NewLegacyManager(config, nil)
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Destroy() })

	return m
}

func TestSystemdVersion(t *testing.T) {
	systemdVersionTests := []struct {
		verStr      string
		expectedVer int
		expectErr   bool
	}{
		{`"219"`, 219, false},
		{`"v245.4-1.fc32"`, 245, false},
		{`"241-1"`, 241, false},
		{`"v241-1"`, 241, false},
		{`333.45"`, 333, false},
		{`v321-0`, 321, false},
		{"NaN", -1, true},
		{"", -1, true},
		{"v", -1, true},
	}
	for _, sdTest := range systemdVersionTests {
		ver, err := systemdVersionAtoi(sdTest.verStr)
		if !sdTest.expectErr && err != nil {
			t.Errorf("systemdVersionAtoi(%s); want nil; got %v", sdTest.verStr, err)
		}
		if sdTest.expectErr && err == nil {
			t.Errorf("systemdVersionAtoi(%s); wanted failure; got nil", sdTest.verStr)
		}
		if ver != sdTest.expectedVer {
			t.Errorf("systemdVersionAtoi(%s); want %d; got %d", sdTest.verStr, sdTest.expectedVer, ver)
		}
	}
}

func TestValidUnitTypes(t *testing.T) {
	testCases := []struct {
		unitName         string
		expectedUnitType string
	}{
		{"system.slice", "Slice"},
		{"kubepods.slice", "Slice"},
		{"testing-container:ab.scope", "Scope"},
	}
	for _, sdTest := range testCases {
		unitType := getUnitType(sdTest.unitName)
		if unitType != sdTest.expectedUnitType {
			t.Errorf("getUnitType(%s); want %q; got %q", sdTest.unitName, sdTest.expectedUnitType, unitType)
		}
	}
}

func TestUnitExistsIgnored(t *testing.T) {
	if !IsRunningSystemd() {
		t.Skip("Test requires systemd.")
	}
	if os.Geteuid() != 0 {
		t.Skip("Test requires root.")
	}

	podConfig := &cgroups.Cgroup{
		Parent:    "system.slice",
		Name:      "system-runc_test_exists.slice",
		Resources: &cgroups.Resources{},
	}
	// Create "pods" cgroup (a systemd slice to hold containers).
	pm := newManager(t, podConfig)

	// create twice to make sure "UnitExists" error is ignored.
	for range 2 {
		if err := pm.Apply(-1); err != nil {
			t.Fatal(err)
		}
	}
}

func TestUnifiedResToSystemdProps(t *testing.T) {
	if !IsRunningSystemd() {
		t.Skip("Test requires systemd.")
	}
	if !cgroups.IsCgroup2UnifiedMode() {
		t.Skip("cgroup v2 is required")
	}

	cm := newDbusConnManager(os.Geteuid() != 0)

	testCases := []struct {
		name     string
		minVer   int
		res      map[string]string
		expError bool
		expProps []systemdDbus.Property
	}{
		{
			name: "empty map",
			res:  map[string]string{},
		},
		{
			name:   "only cpu.idle=1",
			minVer: cpuIdleSupportedVersion,
			res: map[string]string{
				"cpu.idle": "1",
			},
			expProps: []systemdDbus.Property{
				newProp("CPUWeight", uint64(0)),
			},
		},
		{
			name:   "only cpu.idle=0",
			minVer: cpuIdleSupportedVersion,
			res: map[string]string{
				"cpu.idle": "0",
			},
		},
		{
			name:   "cpu.idle=1 and cpu.weight=1000",
			minVer: cpuIdleSupportedVersion,
			res: map[string]string{
				"cpu.idle":   "1",
				"cpu.weight": "1000",
			},
			expProps: []systemdDbus.Property{
				newProp("CPUWeight", uint64(0)),
			},
		},
		{
			name:   "cpu.idle=0 and cpu.weight=1000",
			minVer: cpuIdleSupportedVersion,
			res: map[string]string{
				"cpu.idle":   "0",
				"cpu.weight": "1000",
			},
			expProps: []systemdDbus.Property{
				newProp("CPUWeight", uint64(1000)),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.minVer != 0 && systemdVersion(cm) < tc.minVer {
				t.Skipf("requires systemd >= %d", tc.minVer)
			}
			props, err := unifiedResToSystemdProps(cm, tc.res)
			if err != nil && !tc.expError {
				t.Fatalf("expected no error, got: %v", err)
			}
			if err == nil && tc.expError {
				t.Fatal("expected error, got nil")
			}
			if !reflect.DeepEqual(tc.expProps, props) {
				t.Errorf("wrong properties (exp %+v, got %+v)", tc.expProps, props)
			}
		})
	}
}

func TestAddCPUQuota(t *testing.T) {
	if !IsRunningSystemd() {
		t.Skip("Test requires systemd.")
	}

	cm := newDbusConnManager(os.Geteuid() != 0)

	testCases := []struct {
		name                       string
		quota                      int64
		period                     uint64
		expectedCPUQuotaPerSecUSec uint64
		expectedQuota              int64
	}{
		{
			name:                       "No round up",
			quota:                      500000,
			period:                     1000000,
			expectedCPUQuotaPerSecUSec: 500000,
			expectedQuota:              500000,
		},
		{
			name:                       "With fraction",
			quota:                      123456,
			expectedCPUQuotaPerSecUSec: 1240000,
			expectedQuota:              124000,
		},
		{
			name:                       "Round up at division",
			quota:                      500000,
			period:                     900000,
			expectedCPUQuotaPerSecUSec: 560000,
			expectedQuota:              504000,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			props := []systemdDbus.Property{}
			addCPUQuota(cm, &props, &tc.quota, tc.period)
			var cpuQuotaPerSecUSec uint64
			for _, p := range props {
				if p.Name == "CPUQuotaPerSecUSec" {
					if err := p.Value.Store(&cpuQuotaPerSecUSec); err != nil {
						t.Errorf("failed to parse CPUQuotaPerSecUSec: %v", err)
					}
				}
			}
			if cpuQuotaPerSecUSec != tc.expectedCPUQuotaPerSecUSec {
				t.Errorf("CPUQuotaPerSecUSec is not set as expected (exp: %v, got: %v)", tc.expectedCPUQuotaPerSecUSec, cpuQuotaPerSecUSec)
			}
			if tc.quota != tc.expectedQuota {
				t.Errorf("quota is not updated as expected (exp: %v, got: %v)", tc.expectedQuota, tc.quota)
			}
		})
	}
}
