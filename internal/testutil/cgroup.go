// Package testutil contains helpers for tests.
package testutil

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"golang.org/x/sys/unix"
)

const cgroupRoot = "/sys/fs/cgroup"

var (
	versionOnce sync.Once
	version     int
)

// CgroupVersion returns the version of cgroup mounted at /sys/fs/cgroup:
// 2 for cgroup v2 unified mode, 1 for cgroup v1 (including hybrid mode),
// or 0 if no cgroup filesystem is found there (as it is the case in some
// build environments, such as chroot-based ones).
//
// Unlike [cgroups.IsCgroup2UnifiedMode], it does not assume cgroup v1
// if cgroup v2 is not found.
func CgroupVersion() int {
	versionOnce.Do(func() {
		var st unix.Statfs_t
		if err := unix.Statfs(cgroupRoot, &st); err != nil {
			return
		}
		switch st.Type {
		case unix.CGROUP2_SUPER_MAGIC:
			version = 2
			return
		case unix.TMPFS_MAGIC:
		default:
			return
		}
		// Look for at least one cgroup v1 controller mount.
		ents, err := os.ReadDir(cgroupRoot)
		if err != nil {
			return
		}
		for _, e := range ents {
			if err := unix.Statfs(filepath.Join(cgroupRoot, e.Name()), &st); err != nil {
				continue
			}
			if st.Type == unix.CGROUP_SUPER_MAGIC {
				version = 1
				return
			}
		}
	})
	return version
}

// RequireCgroups skips the test if there is no cgroup v1 or v2 mounted
// at /sys/fs/cgroup.
func RequireCgroups(t testing.TB) {
	t.Helper()
	if CgroupVersion() == 0 {
		t.Skip("no cgroup v1 or v2 found at " + cgroupRoot)
	}
}

// RequireCgroupV1 skips the test unless cgroup v1 is mounted at /sys/fs/cgroup.
func RequireCgroupV1(t testing.TB) {
	t.Helper()
	if CgroupVersion() != 1 {
		t.Skip("requires cgroup v1")
	}
}
