package fs

import (
	"testing"

	"github.com/opencontainers/cgroups"
)

// pointerTo returns a pointer to the given controller value.
func pointerTo(c cgroups.Controller) *cgroups.Controller {
	return &c
}

func TestStats(t *testing.T) {
	testCases := []struct {
		name       string
		controller *cgroups.Controller
		subsystems map[string]map[string]string // subsystem -> file contents
		validate   func(*testing.T, *cgroups.Stats)
	}{
		{
			name:       "CPU stats",
			controller: pointerTo(cgroups.CPU),
			subsystems: map[string]map[string]string{
				"cpu": {
					"cpu.stat": "nr_periods 2000\nnr_throttled 200\nthrottled_time 18446744073709551615\n",
				},
				"cpuacct": {
					"cpuacct.usage":        cpuAcctUsageContents,
					"cpuacct.usage_percpu": cpuAcctUsagePerCPUContents,
					"cpuacct.stat":         cpuAcctStatContents,
				},
			},
			validate: func(t *testing.T, stats *cgroups.Stats) {
				// Verify throttling data from cpu.stat
				expectedThrottling := cgroups.ThrottlingData{
					Periods:          2000,
					ThrottledPeriods: 200,
					ThrottledTime:    18446744073709551615,
				}
				expectThrottlingDataEquals(t, expectedThrottling, stats.CpuStats.ThrottlingData)

				// Verify total usage from cpuacct.usage
				if stats.CpuStats.CpuUsage.TotalUsage != 12262454190222160 {
					t.Errorf("expected TotalUsage 12262454190222160, got %d", stats.CpuStats.CpuUsage.TotalUsage)
				}
			},
		},
		{
			name:       "Memory stats",
			controller: pointerTo(cgroups.Memory),
			subsystems: map[string]map[string]string{
				"memory": {
					"memory.stat":               memoryStatContents,
					"memory.usage_in_bytes":     "2048",
					"memory.max_usage_in_bytes": "4096",
					"memory.failcnt":            "100",
					"memory.limit_in_bytes":     "8192",
					"memory.use_hierarchy":      "1",
				},
			},
			validate: func(t *testing.T, stats *cgroups.Stats) {
				expected := cgroups.MemoryData{Usage: 2048, MaxUsage: 4096, Failcnt: 100, Limit: 8192}
				expectMemoryDataEquals(t, expected, stats.MemoryStats.Usage)
			},
		},
		{
			name:       "Pids stats",
			controller: pointerTo(cgroups.Pids),
			subsystems: map[string]map[string]string{
				"pids": {
					"pids.current": "1337",
					"pids.max":     "1024",
				},
			},
			validate: func(t *testing.T, stats *cgroups.Stats) {
				if stats.PidsStats.Current != 1337 {
					t.Errorf("expected Current 1337, got %d", stats.PidsStats.Current)
				}
				if stats.PidsStats.Limit != 1024 {
					t.Errorf("expected Limit 1024, got %d", stats.PidsStats.Limit)
				}
			},
		},
		{
			name:       "IO stats",
			controller: pointerTo(cgroups.IO),
			subsystems: map[string]map[string]string{
				"blkio": blkioBFQStatsTestFiles,
			},
			validate: func(t *testing.T, stats *cgroups.Stats) {
				// Verify we have entries
				if len(stats.BlkioStats.IoServiceBytesRecursive) == 0 {
					t.Error("expected IoServiceBytesRecursive to have entries")
				}
				if len(stats.BlkioStats.IoServicedRecursive) == 0 {
					t.Error("expected IoServicedRecursive to have entries")
				}
			},
		},
		{
			name:       "Multiple controllers - CPU+Pids",
			controller: pointerTo(cgroups.CPU | cgroups.Pids),
			subsystems: map[string]map[string]string{
				"cpu": {
					"cpu.stat": "nr_periods 100\nnr_throttled 10\nthrottled_time 5000\n",
				},
				"pids": {
					"pids.current": "42",
					"pids.max":     "1000",
				},
			},
			validate: func(t *testing.T, stats *cgroups.Stats) {
				// Verify both are populated
				if stats.CpuStats.ThrottlingData.Periods != 100 {
					t.Errorf("expected Periods 100, got %d", stats.CpuStats.ThrottlingData.Periods)
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
			controller: nil, // nil means all controllers (default behavior)
			subsystems: map[string]map[string]string{
				"cpu": {
					"cpu.stat": "nr_periods 2000\nnr_throttled 200\nthrottled_time 18446744073709551615\n",
				},
				"cpuacct": {
					"cpuacct.usage":        cpuAcctUsageContents,
					"cpuacct.usage_percpu": cpuAcctUsagePerCPUContents,
					"cpuacct.stat":         cpuAcctStatContents,
				},
				"memory": {
					"memory.stat":               memoryStatContents,
					"memory.usage_in_bytes":     "2048",
					"memory.max_usage_in_bytes": "4096",
					"memory.failcnt":            "100",
					"memory.limit_in_bytes":     "8192",
					"memory.use_hierarchy":      "1",
				},
				"pids": {
					"pids.current": "1337",
					"pids.max":     "1024",
				},
				"blkio": blkioBFQStatsTestFiles,
			},
			validate: func(t *testing.T, stats *cgroups.Stats) {
				// Verify CPU stats
				expectedThrottling := cgroups.ThrottlingData{
					Periods:          2000,
					ThrottledPeriods: 200,
					ThrottledTime:    18446744073709551615,
				}
				expectThrottlingDataEquals(t, expectedThrottling, stats.CpuStats.ThrottlingData)
				if stats.CpuStats.CpuUsage.TotalUsage != 12262454190222160 {
					t.Errorf("expected TotalUsage 12262454190222160, got %d", stats.CpuStats.CpuUsage.TotalUsage)
				}

				// Verify Memory stats
				expectedMemory := cgroups.MemoryData{Usage: 2048, MaxUsage: 4096, Failcnt: 100, Limit: 8192}
				expectMemoryDataEquals(t, expectedMemory, stats.MemoryStats.Usage)

				// Verify Pids stats
				if stats.PidsStats.Current != 1337 {
					t.Errorf("expected Current 1337, got %d", stats.PidsStats.Current)
				}
				if stats.PidsStats.Limit != 1024 {
					t.Errorf("expected Limit 1024, got %d", stats.PidsStats.Limit)
				}

				// Verify IO stats
				if len(stats.BlkioStats.IoServiceBytesRecursive) == 0 {
					t.Error("expected IoServiceBytesRecursive to have entries")
				}
				if len(stats.BlkioStats.IoServicedRecursive) == 0 {
					t.Error("expected IoServicedRecursive to have entries")
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create temp directories for each subsystem and write files
			paths := make(map[string]string)
			for subsystem, files := range tc.subsystems {
				path := tempDir(t, subsystem)
				writeFileContents(t, path, files)
				paths[subsystem] = path
			}
			m := &Manager{
				cgroups: &cgroups.Cgroup{Resources: &cgroups.Resources{}},
				paths:   paths,
			}

			var stats *cgroups.Stats
			var err error
			if tc.controller != nil {
				stats, err = m.Stats(&cgroups.StatsOptions{Controllers: *tc.controller})
			} else {
				stats, err = m.Stats(nil)
			}
			if err != nil {
				t.Fatal(err)
			}

			// Validate the results
			tc.validate(t, stats)
		})
	}
}

func BenchmarkGetStats(b *testing.B) {
	if cgroups.IsCgroup2UnifiedMode() {
		b.Skip("cgroup v2 is not supported")
	}

	// Unset TestMode as we work with real cgroupfs here,
	// and we want OpenFile to perform the fstype check.
	cgroups.TestMode = false
	defer func() {
		cgroups.TestMode = true
	}()

	cg := &cgroups.Cgroup{
		Path:      "/some/kind/of/a/path/here",
		Resources: &cgroups.Resources{},
	}
	m, err := NewManager(cg, nil)
	if err != nil {
		b.Fatal(err)
	}
	err = m.Apply(-1)
	if err != nil {
		b.Fatal(err)
	}
	defer func() {
		_ = m.Destroy()
	}()

	var st *cgroups.Stats

	for b.Loop() {
		st, err = m.GetStats()
		if err != nil {
			b.Fatal(err)
		}
	}
	if st.CpuStats.CpuUsage.TotalUsage != 0 {
		b.Fatalf("stats: %+v", st)
	}
}
