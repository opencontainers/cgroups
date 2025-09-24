package fs2

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/opencontainers/cgroups"
)

const exampleIoStatData = `254:1 rbytes=6901432320 wbytes=14245535744 rios=263278 wios=248603 dbytes=0 dios=0
254:0 rbytes=2702336 wbytes=0 rios=97 wios=0 dbytes=0 dios=0
259:0 rbytes=6911345664 wbytes=14245536256 rios=264538 wios=244914 dbytes=530485248 dios=2`

const exampleIoCostDebugData = `251:0 rbytes=2285568 wbytes=688128 rios=107 wios=168 dbytes=0 dios=0
252:0 rbytes=2037743988736 wbytes=1036567117824 rios=169193849 wios=41541021 dbytes=1012840136704 dios=199909
259:0 rbytes=4085926524416 wbytes=1036680064512 rios=185034771 wios=40358485 dbytes=1013982564352 dios=199959 cost.vrate=100.00 cost.usage=1532009788 cost.wait=1477289869 cost.indebt=0 cost.indelay=0`

var exampleIoStatsParsed = cgroups.BlkioStats{
	IoServiceBytesRecursive: []cgroups.BlkioStatEntry{
		{Major: 254, Minor: 1, Value: 6901432320, Op: "Read"},
		{Major: 254, Minor: 1, Value: 14245535744, Op: "Write"},
		{Major: 254, Minor: 0, Value: 2702336, Op: "Read"},
		{Major: 254, Minor: 0, Value: 0, Op: "Write"},
		{Major: 259, Minor: 0, Value: 6911345664, Op: "Read"},
		{Major: 259, Minor: 0, Value: 14245536256, Op: "Write"},
	},
	IoServicedRecursive: []cgroups.BlkioStatEntry{
		{Major: 254, Minor: 1, Value: 263278, Op: "Read"},
		{Major: 254, Minor: 1, Value: 248603, Op: "Write"},
		{Major: 254, Minor: 0, Value: 97, Op: "Read"},
		{Major: 254, Minor: 0, Value: 0, Op: "Write"},
		{Major: 259, Minor: 0, Value: 264538, Op: "Read"},
		{Major: 259, Minor: 0, Value: 244914, Op: "Write"},
	},
}

var exampleIoCostDebugParsed = cgroups.BlkioStats{
	IoServiceBytesRecursive: []cgroups.BlkioStatEntry{
		{Major: 251, Minor: 0, Value: 2285568, Op: "Read"},
		{Major: 251, Minor: 0, Value: 688128, Op: "Write"},
		{Major: 252, Minor: 0, Value: 2037743988736, Op: "Read"},
		{Major: 252, Minor: 0, Value: 1036567117824, Op: "Write"},
		{Major: 259, Minor: 0, Value: 4085926524416, Op: "Read"},
		{Major: 259, Minor: 0, Value: 1036680064512, Op: "Write"},
	},
	IoServicedRecursive: []cgroups.BlkioStatEntry{
		{Major: 251, Minor: 0, Value: 107, Op: "Read"},
		{Major: 251, Minor: 0, Value: 168, Op: "Write"},
		{Major: 252, Minor: 0, Value: 169193849, Op: "Read"},
		{Major: 252, Minor: 0, Value: 41541021, Op: "Write"},
		{Major: 259, Minor: 0, Value: 185034771, Op: "Read"},
		{Major: 259, Minor: 0, Value: 40358485, Op: "Write"},
	},
	IoCostUsage: []cgroups.BlkioStatEntry{
		{Major: 259, Minor: 0, Value: 1532009788, Op: "Count"},
	},
	IoCostWait: []cgroups.BlkioStatEntry{
		{Major: 259, Minor: 0, Value: 1477289869, Op: "Count"},
	},
	IoCostIndebt: []cgroups.BlkioStatEntry{
		{Major: 259, Minor: 0, Value: 0, Op: "Count"},
	},
	IoCostIndelay: []cgroups.BlkioStatEntry{
		{Major: 259, Minor: 0, Value: 0, Op: "Count"},
	},
}

func lessBlkioStatEntry(a, b cgroups.BlkioStatEntry) bool {
	if a.Major != b.Major {
		return a.Major < b.Major
	}
	if a.Minor != b.Minor {
		return a.Minor < b.Minor
	}
	if a.Op != b.Op {
		return a.Op < b.Op
	}
	return a.Value < b.Value
}

func sortBlkioStats(stats *cgroups.BlkioStats) {
	for _, table := range []*[]cgroups.BlkioStatEntry{
		&stats.IoServicedRecursive,
		&stats.IoServiceBytesRecursive,
	} {
		sort.SliceStable(*table, func(i, j int) bool { return lessBlkioStatEntry((*table)[i], (*table)[j]) })
	}
}

func TestStatIo(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected cgroups.BlkioStats
	}{
		{
			name:     "default io.stat case",
			input:    exampleIoStatData,
			expected: exampleIoStatsParsed,
		},
		{
			name:     "io.stat with iocost debug data",
			input:    exampleIoCostDebugData,
			expected: exampleIoCostDebugParsed,
		},
	}

	// We're using a fake cgroupfs.
	cgroups.TestMode = true

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fakeCgroupDir := t.TempDir()
			statPath := filepath.Join(fakeCgroupDir, "io.stat")

			if err := os.WriteFile(statPath, []byte(tt.input), 0o644); err != nil {
				t.Fatal(err)
			}

			var gotStats cgroups.Stats
			if err := statIo(fakeCgroupDir, &gotStats); err != nil {
				t.Error(err)
			}

			// Sort the output since statIo uses a map internally.
			sortBlkioStats(&gotStats.BlkioStats)
			sortBlkioStats(&tt.expected)

			if !reflect.DeepEqual(gotStats.BlkioStats, tt.expected) {
				t.Errorf("parsed cgroupv2 io.stat doesn't match expected result: \ngot %#v\nexpected %#v\n", gotStats.BlkioStats, tt.expected)
			}
		})
	}
}
