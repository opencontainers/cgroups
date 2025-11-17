package fs

import (
	"strings"
	"testing"

	"github.com/opencontainers/cgroups"
)

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

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		st, err = m.GetStats()
		if err != nil {
			b.Fatal(err)
		}
	}
	if st.CpuStats.CpuUsage.TotalUsage != 0 {
		b.Fatalf("stats: %+v", st)
	}
}

func TestAddCpuStats(t *testing.T) {
	cpuPath := tempDir(t, "cpu")
	cpuacctPath := tempDir(t, "cpuacct")

	writeFileContents(t, cpuPath, map[string]string{
		"cpu.stat": "nr_periods 2000\nnr_throttled 200\nthrottled_time 18446744073709551615\n",
	})
	writeFileContents(t, cpuacctPath, map[string]string{
		"cpuacct.usage":        cpuAcctUsageContents,
		"cpuacct.usage_percpu": cpuAcctUsagePerCPUContents,
		"cpuacct.stat":         cpuAcctStatContents,
	})

	m := &Manager{
		cgroups: &cgroups.Cgroup{Resources: &cgroups.Resources{}},
		paths:   map[string]string{"cpu": cpuPath, "cpuacct": cpuacctPath},
	}

	stats := cgroups.NewStats()
	if err := m.AddCpuStats(stats); err != nil {
		t.Fatal(err)
	}

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
}

func TestAddPidsStats(t *testing.T) {
	path := tempDir(t, "pids")
	writeFileContents(t, path, map[string]string{
		"pids.current": "1337",
		"pids.max":     "1024",
	})

	m := &Manager{
		cgroups: &cgroups.Cgroup{Resources: &cgroups.Resources{}},
		paths:   map[string]string{"pids": path},
	}

	stats := cgroups.NewStats()
	if err := m.AddPidsStats(stats); err != nil {
		t.Fatal(err)
	}

	if stats.PidsStats.Current != 1337 {
		t.Errorf("expected Current 1337, got %d", stats.PidsStats.Current)
	}
	if stats.PidsStats.Limit != 1024 {
		t.Errorf("expected Limit 1024, got %d", stats.PidsStats.Limit)
	}
}

func TestAddMemoryStats(t *testing.T) {
	path := tempDir(t, "memory")
	writeFileContents(t, path, map[string]string{
		"memory.stat":               memoryStatContents,
		"memory.usage_in_bytes":     "2048",
		"memory.max_usage_in_bytes": "4096",
		"memory.failcnt":            "100",
		"memory.limit_in_bytes":     "8192",
		"memory.use_hierarchy":      "1",
	})

	m := &Manager{
		cgroups: &cgroups.Cgroup{Resources: &cgroups.Resources{}},
		paths:   map[string]string{"memory": path},
	}

	stats := cgroups.NewStats()
	if err := m.AddMemoryStats(stats); err != nil {
		t.Fatal(err)
	}

	expected := cgroups.MemoryData{Usage: 2048, MaxUsage: 4096, Failcnt: 100, Limit: 8192}
	expectMemoryDataEquals(t, expected, stats.MemoryStats.Usage)
}

func TestAddIoStats(t *testing.T) {
	path := tempDir(t, "blkio")
	// Use blkioBFQStatsTestFiles from blkio_test.go for proper file format
	writeFileContents(t, path, blkioBFQStatsTestFiles)

	m := &Manager{
		cgroups: &cgroups.Cgroup{Resources: &cgroups.Resources{}},
		paths:   map[string]string{"blkio": path},
	}

	stats := cgroups.NewStats()
	if err := m.AddIoStats(stats); err != nil {
		t.Fatal(err)
	}

	// Verify we have entries
	if len(stats.BlkioStats.IoServiceBytesRecursive) == 0 {
		t.Error("expected IoServiceBytesRecursive to have entries")
	}
	if len(stats.BlkioStats.IoServicedRecursive) == 0 {
		t.Error("expected IoServicedRecursive to have entries")
	}
}

func TestAddStatsIterative(t *testing.T) {
	// Set up both cpu and pids directories
	cpuPath := tempDir(t, "cpu")
	pidsPath := tempDir(t, "pids")

	writeFileContents(t, cpuPath, map[string]string{
		"cpu.stat": "nr_periods 100\nnr_throttled 10\nthrottled_time 5000\n",
	})
	writeFileContents(t, pidsPath, map[string]string{
		"pids.current": "42",
		"pids.max":     "1000",
	})

	m := &Manager{
		cgroups: &cgroups.Cgroup{Resources: &cgroups.Resources{}},
		paths:   map[string]string{"cpu": cpuPath, "pids": pidsPath},
	}

	stats := cgroups.NewStats()

	// Call both methods on same stats object
	if err := m.AddCpuStats(stats); err != nil {
		t.Fatal(err)
	}
	if err := m.AddPidsStats(stats); err != nil {
		t.Fatal(err)
	}

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
}

// TestAddStatsWithEmptyPaths tests that Add*Stats methods work correctly
// when the corresponding controller paths are empty (controller not available).
func TestAddStatsWithEmptyPaths(t *testing.T) {
	m := &Manager{
		cgroups: &cgroups.Cgroup{Resources: &cgroups.Resources{}},
		paths:   make(map[string]string),
	}

	stats := cgroups.NewStats()

	// All Add*Stats methods should succeed with empty paths (no-op)
	if err := m.AddCpuStats(stats); err != nil {
		t.Errorf("AddCpuStats failed with empty paths: %v", err)
	}
	if err := m.AddMemoryStats(stats); err != nil {
		t.Errorf("AddMemoryStats failed with empty paths: %v", err)
	}
	if err := m.AddPidsStats(stats); err != nil {
		t.Errorf("AddPidsStats failed with empty paths: %v", err)
	}
	if err := m.AddIoStats(stats); err != nil {
		t.Errorf("AddIoStats failed with empty paths: %v", err)
	}
	if err := m.AddHugetlbStats(stats); err != nil {
		t.Errorf("AddHugetlbStats failed with empty paths: %v", err)
	}
	if err := m.AddRdmaStats(stats); err != nil {
		t.Errorf("AddRdmaStats failed with empty paths: %v", err)
	}
	if err := m.AddMiscStats(stats); err != nil {
		t.Errorf("AddMiscStats failed with empty paths: %v", err)
	}
}

// TestAddStatsValidation tests that Add*Stats methods properly validate
// nil parameters and nil maps.
func TestAddStatsValidation(t *testing.T) {
	m := &Manager{
		cgroups: &cgroups.Cgroup{Resources: &cgroups.Resources{}},
		paths:   make(map[string]string),
	}

	// Create manager with paths for map validation tests
	tempDir := t.TempDir()
	mWithPaths := &Manager{
		cgroups: &cgroups.Cgroup{Resources: &cgroups.Resources{}},
		paths: map[string]string{
			"memory":  tempDir,
			"hugetlb": tempDir,
		},
	}

	// Create stats with nil maps for map validation tests
	statsWithNilMaps := &cgroups.Stats{}

	tests := []struct {
		name        string
		manager     *Manager
		stats       *cgroups.Stats
		fn          func(*Manager, *cgroups.Stats) error
		expectedErr string
	}{
		// Nil stats parameter tests
		{"AddCpuStats with nil stats", m, nil, (*Manager).AddCpuStats, cgroups.ErrStatsNil},
		{"AddMemoryStats with nil stats", m, nil, (*Manager).AddMemoryStats, cgroups.ErrStatsNil},
		{"AddPidsStats with nil stats", m, nil, (*Manager).AddPidsStats, cgroups.ErrStatsNil},
		{"AddIoStats with nil stats", m, nil, (*Manager).AddIoStats, cgroups.ErrStatsNil},
		{"AddHugetlbStats with nil stats", m, nil, (*Manager).AddHugetlbStats, cgroups.ErrStatsNil},
		{"AddRdmaStats with nil stats", m, nil, (*Manager).AddRdmaStats, cgroups.ErrStatsNil},
		{"AddMiscStats with nil stats", m, nil, (*Manager).AddMiscStats, cgroups.ErrStatsNil},

		// Nil map tests
		{"AddMemoryStats with nil Stats map", mWithPaths, statsWithNilMaps, (*Manager).AddMemoryStats, "stats.MemoryStats.Stats must not be nil"},
		{"AddHugetlbStats with nil HugetlbStats map", mWithPaths, statsWithNilMaps, (*Manager).AddHugetlbStats, "stats.HugetlbStats must not be nil"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn(tt.manager, tt.stats)
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


