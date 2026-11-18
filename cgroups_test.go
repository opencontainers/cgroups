package cgroups

import (
	"testing"

	"github.com/opencontainers/cgroups/internal/testutil"
)

func TestParseCgroups(t *testing.T) {
	// We don't need to use /proc/thread-self here because runc always runs
	// with every thread in the same cgroup. This lets us avoid having to do
	// runtime.LockOSThread.
	cgroups, err := ParseCgroupFile("/proc/self/cgroup")
	if err != nil {
		t.Fatal(err)
	}
	if testutil.CgroupVersion() != 1 {
		return
	}
	if _, ok := cgroups["cpu"]; !ok {
		t.Fail()
	}
}
