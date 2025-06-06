/*
Utility for testing cgroup operations.

Creates a mock of the cgroup filesystem for the duration of the test.
*/
package fs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/opencontainers/cgroups"
)

func init() {
	cgroups.TestMode = true
}

// tempDir creates a new test directory for the specified subsystem.
func tempDir(t testing.TB, subsystem string) string {
	path := filepath.Join(t.TempDir(), subsystem)
	// Ensure the full mock cgroup path exists.
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// writeFileContents writes the specified contents on the mock of the specified
// cgroup files.
func writeFileContents(t testing.TB, path string, fileContents map[string]string) {
	for file, contents := range fileContents {
		err := cgroups.WriteFile(path, file, contents)
		if err != nil {
			t.Fatal(err)
		}
	}
}
