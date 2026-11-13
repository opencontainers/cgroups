package cgroups

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestWriteCgroupFileHandlesInterrupt(t *testing.T) {
	const (
		memoryCgroupMount = "/sys/fs/cgroup/memory"
		memoryLimit       = "memory.limit_in_bytes"
	)
	if _, err := os.Stat(memoryCgroupMount); err != nil {
		// most probably cgroupv2
		t.Skip(err)
	}

	cgroupName := fmt.Sprintf("test-eint-%d", time.Now().Nanosecond())
	cgroupPath := filepath.Join(memoryCgroupMount, cgroupName)
	if err := os.MkdirAll(cgroupPath, 0o755); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(cgroupPath)

	if _, err := os.Stat(filepath.Join(cgroupPath, memoryLimit)); err != nil {
		// either cgroupv2, or memory controller is not available
		t.Skip(err)
	}

	for i := range 100000 {
		limit := 1024*1024 + i
		if err := WriteFile(cgroupPath, memoryLimit, strconv.Itoa(limit)); err != nil {
			t.Fatalf("Failed to write %d on attempt %d: %+v", limit, i, err)
		}
	}
}

func TestOpenat2(t *testing.T) {
	if !IsCgroup2UnifiedMode() {
		// The reason is many test cases below test opening files from
		// the top-level directory, where cgroup v1 has no files.
		t.Skip("test requires cgroup v2")
	}

	// Make sure we test openat2, not its fallback.
	openFallback = func(_ string, _ int, _ os.FileMode) (*os.File, error) {
		return nil, errors.New("fallback")
	}
	defer func() { openFallback = openAndCheck }()

	for _, tc := range []struct{ dir, file string }{
		{"/sys/fs/cgroup", "cgroup.controllers"},
		{"/sys/fs/cgroup", "/cgroup.controllers"},
		{"/sys/fs/cgroup/", "cgroup.controllers"},
		{"/sys/fs/cgroup/", "/cgroup.controllers"},
		{"/", "/sys/fs/cgroup/cgroup.controllers"},
		{"/", "sys/fs/cgroup/cgroup.controllers"},
		{"/sys/fs/cgroup/cgroup.controllers", ""},
	} {
		fd, err := OpenFile(tc.dir, tc.file, os.O_RDONLY)
		if err != nil {
			t.Errorf("case %+v: %v", tc, err)
		}
		fd.Close()
	}
}

func TestCgroupRootHandleOpenedToAnotherFile(t *testing.T) {
	const (
		memoryCgroupMount = "/sys/fs/cgroup/memory"
		memoryLimit       = "memory.limit_in_bytes"
	)
	if _, err := os.Stat(memoryCgroupMount); err != nil {
		// most probably cgroupv2
		t.Skip(err)
	}

	cgroupName := fmt.Sprintf("test-eano-%d", time.Now().Nanosecond())
	cgroupPath := filepath.Join(memoryCgroupMount, cgroupName)
	if err := os.MkdirAll(cgroupPath, 0o755); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(cgroupPath)

	if _, err := os.Stat(filepath.Join(cgroupPath, memoryLimit)); err != nil {
		// either cgroupv2, or memory controller is not available
		t.Skip(err)
	}

	// The cgroupRootHandle is opened when the openFile is called.
	if _, err := openFile(cgroupfsDir, filepath.Join("memory", cgroupName, memoryLimit), os.O_RDONLY); err != nil {
		t.Fatal(err)
	}

	// Make sure the cgroupRootHandle is opened to another file.
	if err := syscall.Close(int(cgroupRootHandle.Fd())); err != nil {
		t.Fatal(err)
	}
	if _, err := unix.Openat2(-1, "/tmp", &unix.OpenHow{Flags: unix.O_DIRECTORY | unix.O_PATH | unix.O_CLOEXEC}); err != nil {
		t.Fatal(err)
	}

	var readErr *error
	readErrLock := sync.Mutex{}
	errCount := 0

	// The openFile returns error (may be multiple times) and the prepOnce is reset only once.
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := openFile(cgroupfsDir, filepath.Join("memory", cgroupName, memoryLimit), os.O_RDONLY)
			t.Logf("openFile attempt %d: %v\n", i, err)
			if err != nil {
				readErrLock.Lock()
				readErr = &err
				errCount++
				readErrLock.Unlock()
			}
		}(i)
	}
	wg.Wait()

	if errCount == 0 {
		t.Fatal("At least one openFile should fail")
	}

	if !strings.Contains((*readErr).Error(), "unexpectedly opened to") {
		t.Fatalf("openFile should fail with 'cgroupRootHandle %d unexpectedly opened to <another file>'", cgroupRootHandle.Fd())
	}

	// The openFile should work after prepOnce is reset because the cgroupRootHandle is updated.
	if _, err := openFile(cgroupfsDir, filepath.Join("memory", cgroupName, memoryLimit), os.O_RDONLY); err != nil {
		t.Fatal(err)
	}
}

func BenchmarkWriteFile(b *testing.B) {
	TestMode = true
	defer func() { TestMode = false }()

	dir := b.TempDir()
	tc := []string{
		"one",
		"one\ntwo\nthree",
		"10:200 foo=bar boo=far\n300:1200 something=other\ndefault 45000\n",
		"\n\n\n\n\n\n\n\n",
	}

	for b.Loop() {
		for _, val := range tc {
			if err := WriteFileByLine(dir, "file", val); err != nil {
				b.Fatal(err)
			}
		}
	}
}
