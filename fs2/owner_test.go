package fs2

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/opencontainers/cgroups"
)

func TestOwnerUID(t *testing.T) {
	if !cgroups.IsCgroup2UnifiedMode() {
		t.Skip("Test requires cgroup v2.")
	}
	if os.Geteuid() != 0 {
		t.Skip("Test requires root.")
	}

	const uid = 12345
	ownerUID := uid
	m, err := NewManager(&cgroups.Cgroup{
		Path:      "/cgroups-test-owner-uid",
		Resources: &cgroups.Resources{},
		OwnerUID:  &ownerUID,
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Destroy() })

	if err := m.Apply(-1); err != nil {
		t.Fatal(err)
	}

	path := m.Path("")
	checkOwner := func(file string, want int) {
		t.Helper()
		fi, err := os.Stat(filepath.Join(path, file))
		if err != nil {
			if file != "" && errors.Is(err, os.ErrNotExist) {
				return // Some files might not be present.
			}
			t.Fatal(err)
		}
		if got := int(fi.Sys().(*syscall.Stat_t).Uid); got != want {
			t.Errorf("%s/%s: want owner uid %d, got %d", path, file, want, got)
		}
	}

	// Delegated: the directory itself and the delegatable files.
	for _, f := range []string{"", "cgroup.procs", "cgroup.subtree_control", "cgroup.threads"} {
		checkOwner(f, uid)
	}
	// Not delegated.
	checkOwner("cgroup.controllers", 0)
}
