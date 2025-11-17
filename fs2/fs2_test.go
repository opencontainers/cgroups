package fs2

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencontainers/cgroups"
)

const exampleCpuStatData = `usage_usec 1000000
user_usec 600000
system_usec 400000
nr_periods 100
nr_throttled 10
throttled_usec 50000
nr_bursts 5
burst_usec 10000`

const exampleCpuStatDataShort = `usage_usec 1000000
user_usec 600000
system_usec 400000`

const exampleMemoryCurrent = "4194304"
const exampleMemoryMax = "max"

const examplePSIData = `some avg10=1.00 avg60=2.00 avg300=3.00 total=100000
full avg10=0.50 avg60=1.00 avg300=1.50 total=50000`

const exampleRdmaCurrent = `mlx5_0 hca_handle=10 hca_object=20`


func TestAddCpuStats(t *testing.T) {
	// We're using a fake cgroupfs.
	cgroups.TestMode = true

	fakeCgroupDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(fakeCgroupDir, "cpu.stat"), []byte(exampleCpuStatData), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create manager
	config := &cgroups.Cgroup{}
	m, err := NewManager(config, fakeCgroupDir)
	if err != nil {
		t.Fatal(err)
	}

	// Create stats and call AddCpuStats
	stats := cgroups.NewStats()
	if err := m.AddCpuStats(stats); err != nil {
		t.Fatal(err)
	}

	// Verify CPU stats populated correctly (values are converted from usec to nsec)
	if stats.CpuStats.CpuUsage.TotalUsage != 1000000000 {
		t.Errorf("expected TotalUsage 1000000000, got %d", stats.CpuStats.CpuUsage.TotalUsage)
	}
	if stats.CpuStats.CpuUsage.UsageInUsermode != 600000000 {
		t.Errorf("expected UsageInUsermode 600000000, got %d", stats.CpuStats.CpuUsage.UsageInUsermode)
	}
	if stats.CpuStats.CpuUsage.UsageInKernelmode != 400000000 {
		t.Errorf("expected UsageInKernelmode 400000000, got %d", stats.CpuStats.CpuUsage.UsageInKernelmode)
	}
	if stats.CpuStats.ThrottlingData.Periods != 100 {
		t.Errorf("expected Periods 100, got %d", stats.CpuStats.ThrottlingData.Periods)
	}
	if stats.CpuStats.ThrottlingData.ThrottledPeriods != 10 {
		t.Errorf("expected ThrottledPeriods 10, got %d", stats.CpuStats.ThrottlingData.ThrottledPeriods)
	}
}

func TestAddMemoryStats(t *testing.T) {
	cgroups.TestMode = true

	fakeCgroupDir := t.TempDir()

	// Use exampleMemoryStatData from memory_test.go (file = 6502666240)
	if err := os.WriteFile(filepath.Join(fakeCgroupDir, "memory.stat"), []byte(exampleMemoryStatData), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(fakeCgroupDir, "memory.current"), []byte(exampleMemoryCurrent), 0o644); err != nil {
		t.Fatal(err)
	}

	// memory.max is required by getMemoryDataV2
	if err := os.WriteFile(filepath.Join(fakeCgroupDir, "memory.max"), []byte(exampleMemoryMax), 0o644); err != nil {
		t.Fatal(err)
	}

	config := &cgroups.Cgroup{}
	m, err := NewManager(config, fakeCgroupDir)
	if err != nil {
		t.Fatal(err)
	}

	stats := cgroups.NewStats()
	if err := m.AddMemoryStats(stats); err != nil {
		t.Fatal(err)
	}

	// Verify memory stats
	if stats.MemoryStats.Usage.Usage != 4194304 {
		t.Errorf("expected Usage 4194304, got %d", stats.MemoryStats.Usage.Usage)
	}
	// Cache comes from "file" field in memory.stat (6502666240 from exampleMemoryStatData)
	if stats.MemoryStats.Cache != 6502666240 {
		t.Errorf("expected Cache 6502666240, got %d", stats.MemoryStats.Cache)
	}
}

func TestAddPidsStats(t *testing.T) {
	cgroups.TestMode = true

	fakeCgroupDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(fakeCgroupDir, "pids.current"), []byte("42\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fakeCgroupDir, "pids.max"), []byte("1000\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	config := &cgroups.Cgroup{}
	m, err := NewManager(config, fakeCgroupDir)
	if err != nil {
		t.Fatal(err)
	}

	stats := cgroups.NewStats()
	if err := m.AddPidsStats(stats); err != nil {
		t.Fatal(err)
	}

	if stats.PidsStats.Current != 42 {
		t.Errorf("expected Current 42, got %d", stats.PidsStats.Current)
	}
	if stats.PidsStats.Limit != 1000 {
		t.Errorf("expected Limit 1000, got %d", stats.PidsStats.Limit)
	}
}

func TestAddIoStats(t *testing.T) {
	cgroups.TestMode = true

	fakeCgroupDir := t.TempDir()

	// Use exampleIoStatData from io_test.go
	if err := os.WriteFile(filepath.Join(fakeCgroupDir, "io.stat"), []byte(exampleIoStatData), 0o644); err != nil {
		t.Fatal(err)
	}

	config := &cgroups.Cgroup{}
	m, err := NewManager(config, fakeCgroupDir)
	if err != nil {
		t.Fatal(err)
	}

	stats := cgroups.NewStats()
	if err := m.AddIoStats(stats); err != nil {
		t.Fatal(err)
	}

	// Verify IO stats - check that we have entries
	if len(stats.BlkioStats.IoServiceBytesRecursive) == 0 {
		t.Error("expected IoServiceBytesRecursive to have entries")
	}
	if len(stats.BlkioStats.IoServicedRecursive) == 0 {
		t.Error("expected IoServicedRecursive to have entries")
	}
}

func TestAddMiscStats(t *testing.T) {
	cgroups.TestMode = true

	fakeCgroupDir := t.TempDir()

	// Use exampleMiscCurrentData and exampleMiscEventsData from misc_test.go
	if err := os.WriteFile(filepath.Join(fakeCgroupDir, "misc.current"), []byte(exampleMiscCurrentData), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fakeCgroupDir, "misc.events"), []byte(exampleMiscEventsData), 0o644); err != nil {
		t.Fatal(err)
	}

	config := &cgroups.Cgroup{}
	m, err := NewManager(config, fakeCgroupDir)
	if err != nil {
		t.Fatal(err)
	}

	stats := cgroups.NewStats()
	if err := m.AddMiscStats(stats); err != nil {
		t.Fatal(err)
	}

	// Verify misc stats - exampleMiscCurrentData has res_a, res_b, res_c
	if _, ok := stats.MiscStats["res_a"]; !ok {
		t.Error("expected MiscStats to have 'res_a' entry")
	}
	if _, ok := stats.MiscStats["res_b"]; !ok {
		t.Error("expected MiscStats to have 'res_b' entry")
	}
	if _, ok := stats.MiscStats["res_c"]; !ok {
		t.Error("expected MiscStats to have 'res_c' entry")
	}
}

func TestAddStatsIterative(t *testing.T) {
	cgroups.TestMode = true

	fakeCgroupDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(fakeCgroupDir, "cpu.stat"), []byte(exampleCpuStatDataShort), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(fakeCgroupDir, "pids.current"), []byte("42\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fakeCgroupDir, "pids.max"), []byte("1000\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	config := &cgroups.Cgroup{}
	m, err := NewManager(config, fakeCgroupDir)
	if err != nil {
		t.Fatal(err)
	}

	// Test iterative population - call multiple Add*Stats on the same Stats object
	stats := cgroups.NewStats()

	if err := m.AddCpuStats(stats); err != nil {
		t.Fatal(err)
	}
	if err := m.AddPidsStats(stats); err != nil {
		t.Fatal(err)
	}

	// Verify both stats are populated in the same object
	if stats.CpuStats.CpuUsage.TotalUsage != 1000000000 {
		t.Errorf("expected TotalUsage 1000000000, got %d", stats.CpuStats.CpuUsage.TotalUsage)
	}
	if stats.PidsStats.Current != 42 {
		t.Errorf("expected Current 42, got %d", stats.PidsStats.Current)
	}
	if stats.PidsStats.Limit != 1000 {
		t.Errorf("expected Limit 1000, got %d", stats.PidsStats.Limit)
	}
}

func TestAddCpuStatsWithPSI(t *testing.T) {
	cgroups.TestMode = true

	fakeCgroupDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(fakeCgroupDir, "cpu.stat"), []byte(exampleCpuStatData), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fakeCgroupDir, "cpu.pressure"), []byte(examplePSIData), 0o644); err != nil {
		t.Fatal(err)
	}

	config := &cgroups.Cgroup{}
	m, err := NewManager(config, fakeCgroupDir)
	if err != nil {
		t.Fatal(err)
	}

	stats := cgroups.NewStats()
	if err := m.AddCpuStats(stats); err != nil {
		t.Fatal(err)
	}

	// Verify PSI data is populated
	if stats.CpuStats.PSI == nil {
		t.Fatal("expected PSI to be populated")
	}
	if stats.CpuStats.PSI.Some.Avg10 != 1.00 {
		t.Errorf("expected PSI.Some.Avg10 1.00, got %f", stats.CpuStats.PSI.Some.Avg10)
	}
	if stats.CpuStats.PSI.Full.Total != 50000 {
		t.Errorf("expected PSI.Full.Total 50000, got %d", stats.CpuStats.PSI.Full.Total)
	}
}

func TestAddMemoryStatsWithPSI(t *testing.T) {
	cgroups.TestMode = true

	fakeCgroupDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(fakeCgroupDir, "memory.stat"), []byte(exampleMemoryStatData), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fakeCgroupDir, "memory.current"), []byte(exampleMemoryCurrent), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fakeCgroupDir, "memory.max"), []byte(exampleMemoryMax), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fakeCgroupDir, "memory.pressure"), []byte(examplePSIData), 0o644); err != nil {
		t.Fatal(err)
	}

	config := &cgroups.Cgroup{}
	m, err := NewManager(config, fakeCgroupDir)
	if err != nil {
		t.Fatal(err)
	}

	stats := cgroups.NewStats()
	if err := m.AddMemoryStats(stats); err != nil {
		t.Fatal(err)
	}

	// Verify PSI data is populated
	if stats.MemoryStats.PSI == nil {
		t.Fatal("expected PSI to be populated")
	}
	if stats.MemoryStats.PSI.Some.Avg60 != 2.00 {
		t.Errorf("expected PSI.Some.Avg60 2.00, got %f", stats.MemoryStats.PSI.Some.Avg60)
	}
}

func TestAddIoStatsWithPSI(t *testing.T) {
	cgroups.TestMode = true

	fakeCgroupDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(fakeCgroupDir, "io.stat"), []byte(exampleIoStatData), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fakeCgroupDir, "io.pressure"), []byte(examplePSIData), 0o644); err != nil {
		t.Fatal(err)
	}

	config := &cgroups.Cgroup{}
	m, err := NewManager(config, fakeCgroupDir)
	if err != nil {
		t.Fatal(err)
	}

	stats := cgroups.NewStats()
	if err := m.AddIoStats(stats); err != nil {
		t.Fatal(err)
	}

	// Verify PSI data is populated
	if stats.BlkioStats.PSI == nil {
		t.Fatal("expected PSI to be populated")
	}
	if stats.BlkioStats.PSI.Full.Avg300 != 1.50 {
		t.Errorf("expected PSI.Full.Avg300 1.50, got %f", stats.BlkioStats.PSI.Full.Avg300)
	}
}

func TestAddRdmaStats(t *testing.T) {
	cgroups.TestMode = true

	fakeCgroupDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(fakeCgroupDir, "rdma.current"), []byte(exampleRdmaCurrent), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fakeCgroupDir, "rdma.max"), []byte("mlx5_0 hca_handle=max hca_object=max"), 0o644); err != nil {
		t.Fatal(err)
	}

	config := &cgroups.Cgroup{}
	m, err := NewManager(config, fakeCgroupDir)
	if err != nil {
		t.Fatal(err)
	}

	stats := cgroups.NewStats()
	if err := m.AddRdmaStats(stats); err != nil {
		t.Fatal(err)
	}

	// Verify RDMA stats are populated
	if len(stats.RdmaStats.RdmaCurrent) == 0 {
		t.Error("expected RdmaStats.RdmaCurrent to have entries")
	}
}

func TestAddHugetlbStats(t *testing.T) {
	cgroups.TestMode = true

	fakeCgroupDir := t.TempDir()

	// HugePageSizes() returns available page sizes from the system
	// We can only test if files don't exist (should not error)
	config := &cgroups.Cgroup{}
	m, err := NewManager(config, fakeCgroupDir)
	if err != nil {
		t.Fatal(err)
	}

	stats := cgroups.NewStats()
	// Should not error even when files don't exist
	if err := m.AddHugetlbStats(stats); err != nil {
		t.Fatal(err)
	}
}

// TestAddStatsValidation tests that Add*Stats methods properly validate
// nil parameters and nil maps.
func TestAddStatsValidation(t *testing.T) {
	cgroups.TestMode = true

	fakeCgroupDir := t.TempDir()
	config := &cgroups.Cgroup{}
	m, err := NewManager(config, fakeCgroupDir)
	if err != nil {
		t.Fatal(err)
	}

	// Create stats with nil maps for map validation tests
	statsWithNilMaps := &cgroups.Stats{}

	tests := []struct {
		name        string
		stats       *cgroups.Stats
		fn          func(*cgroups.Stats) error
		expectedErr string
	}{
		// Nil stats parameter tests
		{"AddCpuStats with nil stats", nil, m.AddCpuStats, cgroups.ErrStatsNil},
		{"AddMemoryStats with nil stats", nil, m.AddMemoryStats, cgroups.ErrStatsNil},
		{"AddPidsStats with nil stats", nil, m.AddPidsStats, cgroups.ErrStatsNil},
		{"AddIoStats with nil stats", nil, m.AddIoStats, cgroups.ErrStatsNil},
		{"AddHugetlbStats with nil stats", nil, m.AddHugetlbStats, cgroups.ErrStatsNil},
		{"AddRdmaStats with nil stats", nil, m.AddRdmaStats, cgroups.ErrStatsNil},
		{"AddMiscStats with nil stats", nil, m.AddMiscStats, cgroups.ErrStatsNil},

		// Nil map tests
		{"AddMemoryStats with nil Stats map", statsWithNilMaps, m.AddMemoryStats, "stats.MemoryStats.Stats must not be nil"},
		{"AddHugetlbStats with nil HugetlbStats map", statsWithNilMaps, m.AddHugetlbStats, "stats.HugetlbStats must not be nil"},
		{"AddMiscStats with nil MiscStats map", statsWithNilMaps, m.AddMiscStats, "stats.MiscStats must not be nil"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn(tt.stats)
			if err == nil {
				t.Errorf("expected error containing %q, got nil", tt.expectedErr)
				return
			}
			if !strings.Contains(err.Error(), tt.expectedErr) {
				t.Errorf("expected error containing %q, got %q", tt.expectedErr, err.Error())
			}
		})
	}
}

