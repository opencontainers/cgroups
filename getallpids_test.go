package cgroups

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func BenchmarkGetAllPids(b *testing.B) {
	total := 0
	for b.Loop() {
		i, err := GetAllPids("/sys/fs/cgroup")
		if err != nil {
			b.Fatal(err)
		}
		total += len(i)
	}
	b.Logf("iter: %d, total: %d", b.N, total)
}

func TestGetAllPidsIgnoresVanishedSubcgroup(t *testing.T) {
	TestMode = true
	defer func() { TestMode = false }()

	root := t.TempDir()
	if err := WriteFile(root, "cgroup.procs", "1\n"); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(root, "child")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(child, "cgroup.procs", "2\n3\n"); err != nil {
		t.Fatal(err)
	}

	// Pretend "vanishing" cgroup, directory exists but not the file to read
	if err := os.Mkdir(filepath.Join(root, "gone"), 0o755); err != nil {
		t.Fatal(err)
	}

	pids, err := GetAllPids(root)
	if err != nil {
		t.Fatalf("GetAllPids errored on a vanished child cgroup: %v", err)
	}
	slices.Sort(pids)
	if want := []int{1, 2, 3}; !slices.Equal(pids, want) {
		t.Errorf("GetAllPids() = %v, want %v", pids, want)
	}
}
