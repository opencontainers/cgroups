package fs

import (
	"strconv"
	"testing"

	"github.com/opencontainers/cgroups"
	"github.com/opencontainers/cgroups/fscommon"
)

const (
	maxUnlimited = -1
	maxLimited   = 1024
)

func toPtr[T any](v T) *T { return &v }

func TestPidsSetMax(t *testing.T) {
	path := tempDir(t, "pids")

	writeFileContents(t, path, map[string]string{
		"pids.max": "max",
	})

	r := &cgroups.Resources{
		PidsLimit: toPtr[int64](maxLimited),
	}
	pids := &PidsGroup{}
	if err := pids.Set(path, r); err != nil {
		t.Fatal(err)
	}

	value, err := fscommon.GetCgroupParamUint(path, "pids.max")
	if err != nil {
		t.Fatal(err)
	}
	if value != maxLimited {
		t.Fatalf("Expected %d, got %d for setting pids.max - limited", maxLimited, value)
	}
}

func TestPidsSetZero(t *testing.T) {
	path := tempDir(t, "pids")

	writeFileContents(t, path, map[string]string{
		"pids.max": "max",
	})

	r := &cgroups.Resources{
		PidsLimit: toPtr[int64](0),
	}
	pids := &PidsGroup{}
	if err := pids.Set(path, r); err != nil {
		t.Fatal(err)
	}

	value, err := fscommon.GetCgroupParamUint(path, "pids.max")
	if err != nil {
		t.Fatal(err)
	}
	// See comment in (*PidsGroup).Set for why we set to 1 here.
	if value != 1 {
		t.Fatalf("Expected 1, got %d for setting pids.max = 0", value)
	}
}

func TestPidsUnset(t *testing.T) {
	path := tempDir(t, "pids")

	writeFileContents(t, path, map[string]string{
		"pids.max": "12345",
	})

	r := &cgroups.Resources{
		PidsLimit: nil,
	}
	pids := &PidsGroup{}
	if err := pids.Set(path, r); err != nil {
		t.Fatal(err)
	}

	value, err := fscommon.GetCgroupParamUint(path, "pids.max")
	if err != nil {
		t.Fatal(err)
	}
	if value != 12345 {
		t.Fatalf("Expected 12345, got %d for not setting pids.max", value)
	}
}

func TestPidsSetUnlimited(t *testing.T) {
	path := tempDir(t, "pids")

	writeFileContents(t, path, map[string]string{
		"pids.max": strconv.Itoa(maxLimited),
	})

	r := &cgroups.Resources{
		PidsLimit: toPtr[int64](maxUnlimited),
	}
	pids := &PidsGroup{}
	if err := pids.Set(path, r); err != nil {
		t.Fatal(err)
	}

	value, err := fscommon.GetCgroupParamString(path, "pids.max")
	if err != nil {
		t.Fatal(err)
	}
	if value != "max" {
		t.Fatalf("Expected %s, got %s for setting pids.max - unlimited", "max", value)
	}
}

func TestPidsStats(t *testing.T) {
	path := tempDir(t, "pids")

	writeFileContents(t, path, map[string]string{
		"pids.current": strconv.Itoa(1337),
		"pids.max":     strconv.Itoa(maxLimited),
	})

	pids := &PidsGroup{}
	stats := *cgroups.NewStats()
	if err := pids.GetStats(path, &stats); err != nil {
		t.Fatal(err)
	}

	if stats.PidsStats.Current != 1337 {
		t.Fatalf("Expected %d, got %d for pids.current", 1337, stats.PidsStats.Current)
	}

	if stats.PidsStats.Limit != maxLimited {
		t.Fatalf("Expected %d, got %d for pids.max", maxLimited, stats.PidsStats.Limit)
	}
}

func TestPidsStatsUnlimited(t *testing.T) {
	path := tempDir(t, "pids")

	writeFileContents(t, path, map[string]string{
		"pids.current": strconv.Itoa(4096),
		"pids.max":     "max",
	})

	pids := &PidsGroup{}
	stats := *cgroups.NewStats()
	if err := pids.GetStats(path, &stats); err != nil {
		t.Fatal(err)
	}

	if stats.PidsStats.Current != 4096 {
		t.Fatalf("Expected %d, got %d for pids.current", 4096, stats.PidsStats.Current)
	}

	if stats.PidsStats.Limit != 0 {
		t.Fatalf("Expected %d, got %d for pids.max", 0, stats.PidsStats.Limit)
	}
}
