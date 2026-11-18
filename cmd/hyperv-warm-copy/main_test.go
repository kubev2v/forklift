package main

import (
	"strings"
	"testing"
)

func TestParseDiskArgsFullAndDelta(t *testing.T) {
	got, err := parseDiskArgs(
		"full|/hyperv/vm.vhdx||/dev/block0\n" +
			"delta|/hyperv/vm_A1.avhdx|/hyperv/vm.vhdx|/dev/block1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []diskJob{
		{mode: "full", src: "/hyperv/vm.vhdx", parent: "", dev: "/dev/block0"},
		{mode: "delta", src: "/hyperv/vm_A1.avhdx", parent: "/hyperv/vm.vhdx", dev: "/dev/block1"},
	}
	assertDiskJobs(t, got, want)
}

func TestParseDiskArgsIgnoresBlankLines(t *testing.T) {
	got, err := parseDiskArgs("\n  full|/a.vhdx||/dev/block0  \n\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertDiskJobs(t, got, []diskJob{
		{mode: "full", src: "/a.vhdx", dev: "/dev/block0"},
	})
}

func TestParseDiskArgsErrors(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		wantErr string
	}{
		{"empty payload", "   \n\n", "nothing to copy"},
		{"wrong field count", "full|/a.vhdx|/dev/block0", "want 4 |-separated fields"},
		{"unknown mode", "sync|/a.vhdx||/dev/block0", `unknown mode "sync"`},
		{"missing device", "full|/a.vhdx||", "src and dev are required"},
		{"missing source", "delta||/parent.vhdx|/dev/block0", "src and dev are required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseDiskArgs(tc.in)
			if err == nil {
				t.Fatalf("expected error containing %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not contain %q", err, tc.wantErr)
			}
		})
	}
}

// TestParseDiskArgsReportsLineNumber keeps the error actionable when a plan
// has many disks and only one record is malformed.
func TestParseDiskArgsReportsLineNumber(t *testing.T) {
	_, err := parseDiskArgs(
		"full|/a.vhdx||/dev/block0\n" +
			"full|/b.vhdx||/dev/block1\n" +
			"broken")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "line 3") {
		t.Fatalf("error %q missing line number", err)
	}
}

func assertDiskJobs(t *testing.T, got, want []diskJob) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d jobs, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("job %d: got %+v want %+v", i, got[i], want[i])
		}
	}
}

// qemu-img cannot follow Hyper-V VHDX backing files; full copy must refuse
// AVHDX rather than convert an incomplete leaf.
func TestFullCopyRejectsDifferencingAVHDX(t *testing.T) {
	err := fullCopy(diskJob{
		mode: modeFull,
		src:  "/hyperv/child.avhdx",
		dev:  "/dev/block0",
	})
	if err == nil {
		t.Fatal("expected error for differencing AVHDX")
	}
	if !strings.Contains(err.Error(), "differencing") {
		t.Fatalf("error %q does not mention differencing", err)
	}
}

func TestFullCopyRejectsLegacyAVHD(t *testing.T) {
	err := fullCopy(diskJob{
		mode: modeFull,
		src:  "/hyperv/child.avhd",
		dev:  "/dev/block0",
	})
	if err == nil {
		t.Fatal("expected error for differencing AVHD")
	}
	if !strings.Contains(err.Error(), "differencing") {
		t.Fatalf("error %q does not mention differencing", err)
	}
}
