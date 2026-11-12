package devices

import (
	"bytes"
	"encoding/binary"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/cilium/ebpf/asm"
	devices "github.com/opencontainers/cgroups/devices/config"
)

func hash(s, comm string) string {
	var res []string
	for l := range strings.SplitSeq(s, "\n") {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" || strings.HasPrefix(trimmed, comm) {
			continue
		}
		res = append(res, trimmed)
	}
	return strings.Join(res, "\n")
}

// disassemble decodes the program with cilium/ebpf/asm and labels it the way
// deviceFilter lays it out: a block starts right after the instructions added
// by init and at every jump target, and each jump refers to the block it
// lands on. A jump with a wrong offset lands elsewhere and shows up as a
// misplaced label in the listing.
func disassemble(t testing.TB, insts bpfInstructions, bo binary.ByteOrder) string {
	t.Helper()
	var decoded asm.Instructions
	r := bytes.NewReader(insts.marshal(bo))
	for r.Len() > 0 {
		var ins asm.Instruction
		if err := ins.Unmarshal(r, bo, "linux"); err != nil {
			t.Fatal(err)
		}
		decoded = append(decoded, ins)
	}

	var p program
	p.init()
	starts := []int{len(p.insts)}
	for i, ins := range decoded {
		if ins.OpCode.JumpOp() == asm.JNE {
			starts = append(starts, i+1+int(ins.Offset))
		}
	}
	slices.Sort(starts)
	starts = slices.Compact(starts)
	label := func(i int) string {
		return "block-" + strconv.Itoa(slices.Index(starts, i))
	}
	for i, ins := range decoded {
		if ins.OpCode.JumpOp() == asm.JNE {
			target := i + 1 + int(ins.Offset)
			// Print the jump the way the assembler shows a jump to a label.
			ins.Offset = -1
			decoded[i] = ins.WithReference(label(target))
		}
	}
	for _, i := range starts {
		if i >= len(decoded) {
			t.Fatalf("jump to instruction %d of a %d-instruction program", i, len(decoded))
		}
		decoded[i] = decoded[i].WithSymbol(label(i))
	}
	return decoded.String()
}

func testDeviceFilter(t testing.TB, devices []*devices.Rule, expectedStr string) {
	insts, _, err := deviceFilter(devices)
	if err != nil {
		t.Fatalf("%s: %v (devices: %+v)", t.Name(), err, devices)
	}
	s := disassemble(t, insts, binary.LittleEndian)
	if be := disassemble(t, insts, binary.BigEndian); be != s {
		t.Fatalf("big-endian encoding decodes to:\n%s\nlittle-endian encoding decodes to:\n%s", be, s)
	}
	if expectedStr != "" {
		hashed := hash(s, "//")
		expectedHashed := hash(expectedStr, "//")
		if expectedHashed != hashed {
			t.Fatalf("expected:\n%q\ngot\n%q", expectedHashed, hashed)
		}
	}
}

func TestDeviceFilter_Nil(t *testing.T) {
	expected := `
// load parameters into registers
        0: LdXMemW dst: r2 src: r1 off: 0 imm: 0
        1: AndImm32 dst: r2 imm: 65535
        2: LdXMemW dst: r3 src: r1 off: 0 imm: 0
        3: RShImm32 dst: r3 imm: 16
        4: LdXMemW dst: r4 src: r1 off: 4 imm: 0
        5: LdXMemW dst: r5 src: r1 off: 8 imm: 0
block-0:
// return 0 (reject)
        6: MovImm32 dst: r0 imm: 0
        7: Exit
	`
	testDeviceFilter(t, nil, expected)
}

func TestDeviceFilter_BuiltInAllowList(t *testing.T) {
	// This is a copy of all rules from
	// github.com/opencontainers/runc/libcontainer/specconv.AllowedDevices.
	devices := []*devices.Rule{
		{
			Type:        devices.CharDevice,
			Major:       devices.Wildcard,
			Minor:       devices.Wildcard,
			Permissions: "m",
			Allow:       true,
		},
		{
			Type:        devices.BlockDevice,
			Major:       devices.Wildcard,
			Minor:       devices.Wildcard,
			Permissions: "m",
			Allow:       true,
		},
		{
			Type:        devices.CharDevice,
			Major:       1,
			Minor:       3,
			Permissions: "rwm",
			Allow:       true,
		},
		{
			Type:        devices.CharDevice,
			Major:       1,
			Minor:       8,
			Permissions: "rwm",
			Allow:       true,
		},
		{
			Type:        devices.CharDevice,
			Major:       1,
			Minor:       7,
			Permissions: "rwm",
			Allow:       true,
		},
		{
			Type:        devices.CharDevice,
			Major:       5,
			Minor:       0,
			Permissions: "rwm",
			Allow:       true,
		},
		{
			Type:        devices.CharDevice,
			Major:       1,
			Minor:       5,
			Permissions: "rwm",
			Allow:       true,
		},
		{
			Type:        devices.CharDevice,
			Major:       1,
			Minor:       9,
			Permissions: "rwm",
			Allow:       true,
		},
		{
			Type:        devices.CharDevice,
			Major:       136,
			Minor:       devices.Wildcard,
			Permissions: "rwm",
			Allow:       true,
		},
		{
			Type:        devices.CharDevice,
			Major:       5,
			Minor:       2,
			Permissions: "rwm",
			Allow:       true,
		},
		{
			Type:        devices.CharDevice,
			Major:       10,
			Minor:       200,
			Permissions: "rwm",
			Allow:       true,
		},
	}

	expected := `
// load parameters into registers
        0: LdXMemW dst: r2 src: r1 off: 0 imm: 0
        1: AndImm32 dst: r2 imm: 65535
        2: LdXMemW dst: r3 src: r1 off: 0 imm: 0
        3: RShImm32 dst: r3 imm: 16
        4: LdXMemW dst: r4 src: r1 off: 4 imm: 0
        5: LdXMemW dst: r5 src: r1 off: 8 imm: 0
block-0:
// (b, wildcard, wildcard, m, true)
        6: JNEImm dst: r2 off: -1 imm: 1 <block-1>
        7: MovReg32 dst: r1 src: r3
        8: AndImm32 dst: r1 imm: 1
        9: JNEReg dst: r1 off: -1 src: r3 <block-1>
        10: MovImm32 dst: r0 imm: 1
        11: Exit
block-1:
// (c, wildcard, wildcard, m, true)
        12: JNEImm dst: r2 off: -1 imm: 2 <block-2>
        13: MovReg32 dst: r1 src: r3
        14: AndImm32 dst: r1 imm: 1
        15: JNEReg dst: r1 off: -1 src: r3 <block-2>
        16: MovImm32 dst: r0 imm: 1
        17: Exit
block-2:
        18: JNEImm dst: r2 off: -1 imm: 2 <block-3>
        19: JNEImm dst: r4 off: -1 imm: 1 <block-3>
        20: JNEImm dst: r5 off: -1 imm: 3 <block-3>
        21: MovImm32 dst: r0 imm: 1
        22: Exit
block-3:
        23: JNEImm dst: r2 off: -1 imm: 2 <block-4>
        24: JNEImm dst: r4 off: -1 imm: 1 <block-4>
        25: JNEImm dst: r5 off: -1 imm: 5 <block-4>
        26: MovImm32 dst: r0 imm: 1
        27: Exit
block-4:
        28: JNEImm dst: r2 off: -1 imm: 2 <block-5>
        29: JNEImm dst: r4 off: -1 imm: 1 <block-5>
        30: JNEImm dst: r5 off: -1 imm: 7 <block-5>
        31: MovImm32 dst: r0 imm: 1
        32: Exit
block-5:
        33: JNEImm dst: r2 off: -1 imm: 2 <block-6>
        34: JNEImm dst: r4 off: -1 imm: 1 <block-6>
        35: JNEImm dst: r5 off: -1 imm: 8 <block-6>
        36: MovImm32 dst: r0 imm: 1
        37: Exit
block-6:
        38: JNEImm dst: r2 off: -1 imm: 2 <block-7>
        39: JNEImm dst: r4 off: -1 imm: 1 <block-7>
        40: JNEImm dst: r5 off: -1 imm: 9 <block-7>
        41: MovImm32 dst: r0 imm: 1
        42: Exit
block-7:
        43: JNEImm dst: r2 off: -1 imm: 2 <block-8>
        44: JNEImm dst: r4 off: -1 imm: 5 <block-8>
        45: JNEImm dst: r5 off: -1 imm: 0 <block-8>
        46: MovImm32 dst: r0 imm: 1
        47: Exit
block-8:
        48: JNEImm dst: r2 off: -1 imm: 2 <block-9>
        49: JNEImm dst: r4 off: -1 imm: 5 <block-9>
        50: JNEImm dst: r5 off: -1 imm: 2 <block-9>
        51: MovImm32 dst: r0 imm: 1
        52: Exit
block-9:
// tuntap (c, 10, 200, rwm, true)
        53: JNEImm dst: r2 off: -1 imm: 2 <block-10>
        54: JNEImm dst: r4 off: -1 imm: 10 <block-10>
        55: JNEImm dst: r5 off: -1 imm: 200 <block-10>
        56: MovImm32 dst: r0 imm: 1
        57: Exit
block-10:
// /dev/pts (c, 136, wildcard, rwm, true)
	58: JNEImm dst: r2 off: -1 imm: 2 <block-11>
        59: JNEImm dst: r4 off: -1 imm: 136 <block-11>
        60: MovImm32 dst: r0 imm: 1
        61: Exit
block-11:
        62: MovImm32 dst: r0 imm: 0
        63: Exit
`
	testDeviceFilter(t, devices, expected)
}

func TestDeviceFilter_Privileged(t *testing.T) {
	devices := []*devices.Rule{
		{
			Type:        'a',
			Major:       -1,
			Minor:       -1,
			Permissions: "rwm",
			Allow:       true,
		},
	}
	expected := `
// load parameters into registers
        0: LdXMemW dst: r2 src: r1 off: 0 imm: 0
        1: AndImm32 dst: r2 imm: 65535
        2: LdXMemW dst: r3 src: r1 off: 0 imm: 0
        3: RShImm32 dst: r3 imm: 16
        4: LdXMemW dst: r4 src: r1 off: 4 imm: 0
        5: LdXMemW dst: r5 src: r1 off: 8 imm: 0
block-0:
// return 1 (accept)
        6: MovImm32 dst: r0 imm: 1
        7: Exit
	`
	testDeviceFilter(t, devices, expected)
}

func TestDeviceFilter_PrivilegedExceptSingleDevice(t *testing.T) {
	devices := []*devices.Rule{
		{
			Type:        'a',
			Major:       -1,
			Minor:       -1,
			Permissions: "rwm",
			Allow:       true,
		},
		{
			Type:        'b',
			Major:       8,
			Minor:       0,
			Permissions: "rwm",
			Allow:       false,
		},
	}
	expected := `
// load parameters into registers
         0: LdXMemW dst: r2 src: r1 off: 0 imm: 0
         1: AndImm32 dst: r2 imm: 65535
         2: LdXMemW dst: r3 src: r1 off: 0 imm: 0
         3: RShImm32 dst: r3 imm: 16
         4: LdXMemW dst: r4 src: r1 off: 4 imm: 0
         5: LdXMemW dst: r5 src: r1 off: 8 imm: 0
block-0:
// return 0 (reject) if type==b && major == 8 && minor == 0
         6: JNEImm dst: r2 off: -1 imm: 1 <block-1>
         7: JNEImm dst: r4 off: -1 imm: 8 <block-1>
         8: JNEImm dst: r5 off: -1 imm: 0 <block-1>
         9: MovImm32 dst: r0 imm: 0
        10: Exit
block-1:
// return 1 (accept)
        11: MovImm32 dst: r0 imm: 1
        12: Exit
`
	testDeviceFilter(t, devices, expected)
}

func TestDeviceFilter_Weird(t *testing.T) {
	devices := []*devices.Rule{
		{
			Type:        'b',
			Major:       8,
			Minor:       1,
			Permissions: "rwm",
			Allow:       false,
		},
		{
			Type:        'a',
			Major:       -1,
			Minor:       -1,
			Permissions: "rwm",
			Allow:       true,
		},
		{
			Type:        'b',
			Major:       8,
			Minor:       2,
			Permissions: "rwm",
			Allow:       false,
		},
	}
	// 8/1 is allowed, 8/2 is not allowed.
	// This conforms to runc v1.0.0-rc.9 (cgroup1) behavior.
	expected := `
// load parameters into registers
         0: LdXMemW dst: r2 src: r1 off: 0 imm: 0
         1: AndImm32 dst: r2 imm: 65535
         2: LdXMemW dst: r3 src: r1 off: 0 imm: 0
         3: RShImm32 dst: r3 imm: 16
         4: LdXMemW dst: r4 src: r1 off: 4 imm: 0
         5: LdXMemW dst: r5 src: r1 off: 8 imm: 0
block-0:
// return 0 (reject) if type==b && major == 8 && minor == 2
         6: JNEImm dst: r2 off: -1 imm: 1 <block-1>
         7: JNEImm dst: r4 off: -1 imm: 8 <block-1>
         8: JNEImm dst: r5 off: -1 imm: 2 <block-1>
         9: MovImm32 dst: r0 imm: 0
        10: Exit
block-1:
// return 1 (accept)
        11: MovImm32 dst: r0 imm: 1
        12: Exit
`
	testDeviceFilter(t, devices, expected)
}
