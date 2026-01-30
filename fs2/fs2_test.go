package fs2

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/opencontainers/cgroups"
)

const (
	exampleCPUStatData = `usage_usec 1000000
user_usec 600000
system_usec 400000
nr_periods 100
nr_throttled 10
throttled_usec 50000
nr_bursts 5
burst_usec 10000`

	exampleCPUStatDataShort = `usage_usec 1000000
user_usec 600000
system_usec 400000`

	exampleMemoryCurrent = "4194304"
	exampleMemoryMax     = "max"

	examplePSIData = `some avg10=1.00 avg60=2.00 avg300=3.00 total=100000
full avg10=0.50 avg60=1.00 avg300=1.50 total=50000`

	exampleRdmaCurrent = `mlx5_0 hca_handle=10 hca_object=20`
)

func pointerTo(c cgroups.Controller) *cgroups.Controller {
	return &c
}

func TestStats(t *testing.T) {
	// We're using a fake cgroupfs.
	cgroups.TestMode = true

	testCases := []struct {
		name       string
		controller *cgroups.Controller
		setupFiles map[string]string
		validate   func(*testing.T, *cgroups.Stats)
	}{
		{
			name:       "CPU stats",
			controller: pointerTo(cgroups.CPU),
			setupFiles: map[string]string{
				"cpu.stat": exampleCPUStatData,
			},
			validate: func(t *testing.T, stats *cgroups.Stats) {
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
			},
		},
		{
			name:       "CPU stats with PSI",
			controller: pointerTo(cgroups.CPU),
			setupFiles: map[string]string{
				"cpu.stat":     exampleCPUStatData,
				"cpu.pressure": examplePSIData,
			},
			validate: func(t *testing.T, stats *cgroups.Stats) {
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
			},
		},
		{
			name:       "Memory stats",
			controller: pointerTo(cgroups.Memory),
			setupFiles: map[string]string{
				"memory.stat":    exampleMemoryStatData,
				"memory.current": exampleMemoryCurrent,
				"memory.max":     exampleMemoryMax,
			},
			validate: func(t *testing.T, stats *cgroups.Stats) {
				// Verify memory stats
				if stats.MemoryStats.Usage.Usage != 4194304 {
					t.Errorf("expected Usage 4194304, got %d", stats.MemoryStats.Usage.Usage)
				}
				// Cache comes from "file" field in memory.stat (6502666240 from exampleMemoryStatData)
				if stats.MemoryStats.Cache != 6502666240 {
					t.Errorf("expected Cache 6502666240, got %d", stats.MemoryStats.Cache)
				}
			},
		},
		{
			name:       "Memory stats with PSI",
			controller: pointerTo(cgroups.Memory),
			setupFiles: map[string]string{
				"memory.stat":     exampleMemoryStatData,
				"memory.current":  exampleMemoryCurrent,
				"memory.max":      exampleMemoryMax,
				"memory.pressure": examplePSIData,
			},
			validate: func(t *testing.T, stats *cgroups.Stats) {
				// Verify PSI data is populated
				if stats.MemoryStats.PSI == nil {
					t.Fatal("expected PSI to be populated")
				}
				if stats.MemoryStats.PSI.Some.Avg60 != 2.00 {
					t.Errorf("expected PSI.Some.Avg60 2.00, got %f", stats.MemoryStats.PSI.Some.Avg60)
				}
			},
		},
		{
			name:       "Pids stats",
			controller: pointerTo(cgroups.Pids),
			setupFiles: map[string]string{
				"pids.current": "42\n",
				"pids.max":     "1000\n",
			},
			validate: func(t *testing.T, stats *cgroups.Stats) {
				if stats.PidsStats.Current != 42 {
					t.Errorf("expected Current 42, got %d", stats.PidsStats.Current)
				}
				if stats.PidsStats.Limit != 1000 {
					t.Errorf("expected Limit 1000, got %d", stats.PidsStats.Limit)
				}
			},
		},
		{
			name:       "IO stats",
			controller: pointerTo(cgroups.IO),
			setupFiles: map[string]string{
				"io.stat": exampleIoStatData,
			},
			validate: func(t *testing.T, stats *cgroups.Stats) {
				// Verify IO stats - check that we have entries
				if len(stats.BlkioStats.IoServiceBytesRecursive) == 0 {
					t.Error("expected IoServiceBytesRecursive to have entries")
				}
				if len(stats.BlkioStats.IoServicedRecursive) == 0 {
					t.Error("expected IoServicedRecursive to have entries")
				}
			},
		},
		{
			name:       "IO stats with PSI",
			controller: pointerTo(cgroups.IO),
			setupFiles: map[string]string{
				"io.stat":     exampleIoStatData,
				"io.pressure": examplePSIData,
			},
			validate: func(t *testing.T, stats *cgroups.Stats) {
				// Verify PSI data is populated
				if stats.BlkioStats.PSI == nil {
					t.Fatal("expected PSI to be populated")
				}
				if stats.BlkioStats.PSI.Full.Avg300 != 1.50 {
					t.Errorf("expected PSI.Full.Avg300 1.50, got %f", stats.BlkioStats.PSI.Full.Avg300)
				}
			},
		},
		{
			name:       "Misc stats",
			controller: pointerTo(cgroups.Misc),
			setupFiles: map[string]string{
				"misc.current": exampleMiscCurrentData,
				"misc.events":  exampleMiscEventsData,
			},
			validate: func(t *testing.T, stats *cgroups.Stats) {
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
			},
		},
		{
			name:       "RDMA stats",
			controller: pointerTo(cgroups.RDMA),
			setupFiles: map[string]string{
				"rdma.current": exampleRdmaCurrent,
				"rdma.max":     "mlx5_0 hca_handle=max hca_object=max",
			},
			validate: func(t *testing.T, stats *cgroups.Stats) {
				// Verify RDMA stats are populated
				if len(stats.RdmaStats.RdmaCurrent) == 0 {
					t.Error("expected RdmaStats.RdmaCurrent to have entries")
				}
			},
		},
		{
			name:       "HugeTLB stats",
			controller: pointerTo(cgroups.HugeTLB),
			setupFiles: map[string]string{},
			validate: func(_ *testing.T, _ *cgroups.Stats) {
				// HugePageSizes() returns available page sizes from the system
				// We can only test if files don't exist (should not error)
				// No specific assertions needed - just verifying it doesn't error
			},
		},
		{
			name:       "Multiple controllers - CPU+Pids",
			controller: pointerTo(cgroups.CPU | cgroups.Pids),
			setupFiles: map[string]string{
				"cpu.stat":     exampleCPUStatDataShort,
				"pids.current": "42\n",
				"pids.max":     "1000\n",
			},
			validate: func(t *testing.T, stats *cgroups.Stats) {
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
			},
		},
		{
			name:       "All controllers with nil options",
			controller: nil,
			setupFiles: map[string]string{
				"cpu.stat":       exampleCPUStatData,
				"memory.stat":    exampleMemoryStatData,
				"memory.current": exampleMemoryCurrent,
				"memory.max":     exampleMemoryMax,
				"pids.current":   "42\n",
				"pids.max":       "1000\n",
				"io.stat":        exampleIoStatData,
			},
			validate: func(t *testing.T, stats *cgroups.Stats) {
				// Verify all stats are populated (non-zero values)
				if stats.CpuStats.CpuUsage.TotalUsage == 0 {
					t.Error("expected non-zero CPU TotalUsage")
				}
				if stats.MemoryStats.Usage.Usage == 0 {
					t.Error("expected non-zero Memory Usage")
				}
				if stats.PidsStats.Current == 0 {
					t.Error("expected non-zero Pids Current")
				}
				if len(stats.BlkioStats.IoServiceBytesRecursive) == 0 {
					t.Error("expected non-empty IO stats")
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fakeCgroupDir := t.TempDir()

			// Setup
			for filename, content := range tc.setupFiles {
				if err := os.WriteFile(filepath.Join(fakeCgroupDir, filename), []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			config := &cgroups.Cgroup{}
			m, err := NewManager(config, fakeCgroupDir)
			if err != nil {
				t.Fatal(err)
			}

			var stats *cgroups.Stats
			if tc.controller == nil {
				stats, err = m.Stats(nil)
			} else {
				stats, err = m.Stats(&cgroups.StatsOptions{Controllers: *tc.controller})
			}
			if err != nil {
				t.Fatal(err)
			}

			if tc.validate != nil {
				tc.validate(t, stats)
			}
		})
	}
}
