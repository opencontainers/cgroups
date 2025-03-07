package devices

import (
	"os"
	"path"
	"testing"

	"github.com/moby/sys/userns"

	"github.com/opencontainers/cgroups"
	devices "github.com/opencontainers/cgroups/devices/config"
	"github.com/opencontainers/cgroups/fscommon"
)

func init() {
	testingSkipFinalCheck = true
	cgroups.TestMode = true
}

func TestSetV1Allow(t *testing.T) {
	if userns.RunningInUserNS() {
		t.Skip("userns detected; setV1 does nothing")
	}
	dir := t.TempDir()

	for file, contents := range map[string]string{
		"devices.allow": "",
		"devices.deny":  "",
		"devices.list":  "a *:* rwm",
	} {
		err := os.WriteFile(path.Join(dir, file), []byte(contents), 0o600)
		if err != nil {
			t.Fatal(err)
		}
	}

	r := &cgroups.Resources{
		Devices: []*devices.Rule{
			{
				Type:        devices.CharDevice,
				Major:       1,
				Minor:       5,
				Permissions: devices.Permissions("rwm"),
				Allow:       true,
			},
		},
	}

	if err := setV1(dir, r); err != nil {
		t.Fatal(err)
	}

	// The default deny rule must be written.
	value, err := fscommon.GetCgroupParamString(dir, "devices.deny")
	if err != nil {
		t.Fatal(err)
	}
	if value[0] != 'a' {
		t.Errorf("Got the wrong value (%q), set devices.deny failed.", value)
	}

	// Permitted rule must be written.
	if value, err := fscommon.GetCgroupParamString(dir, "devices.allow"); err != nil {
		t.Fatal(err)
	} else if value != "c 1:5 rwm" {
		t.Errorf("Got the wrong value (%q), set devices.allow failed.", value)
	}
}
