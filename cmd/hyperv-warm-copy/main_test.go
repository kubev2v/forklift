package main

import (
	"strings"
	"testing"
)

func TestParseDiskArgs(t *testing.T) {
	tests := map[string]struct {
		in      string
		want    []diskJob
		wantErr string
	}{
		"full and delta across two disks": {
			in: "full|/hyperv/vm.vhdx||/dev/block0\n" +
				"delta|/hyperv/vm_A1.avhdx|/hyperv/vm.vhdx|/dev/block1",
			want: []diskJob{
				{mode: "full", src: "/hyperv/vm.vhdx", parent: "", dev: "/dev/block0"},
				{mode: "delta", src: "/hyperv/vm_A1.avhdx", parent: "/hyperv/vm.vhdx", dev: "/dev/block1"},
			},
		},
		"blank lines and padding are ignored": {
			in: "\n  full|/a.vhdx||/dev/block0  \n\n",
			want: []diskJob{
				{mode: "full", src: "/a.vhdx", dev: "/dev/block0"},
			},
		},
		"empty payload": {
			in: "   \n\n", wantErr: "nothing to copy",
		},
		"wrong field count": {
			// A path containing "|" would land here rather than being
			// silently truncated into the wrong device.
			in: "full|/a.vhdx|/dev/block0", wantErr: "want 4 |-separated fields",
		},
		"unknown mode": {
			in: "sync|/a.vhdx||/dev/block0", wantErr: `unknown mode "sync"`,
		},
		"missing device": {
			in: "full|/a.vhdx||", wantErr: "src and dev are required",
		},
		"missing source": {
			in: "delta||/parent.vhdx|/dev/block0", wantErr: "src and dev are required",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := parseDiskArgs(tc.in)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got jobs %+v", tc.wantErr, got)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error %q does not contain %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %d jobs, want %d: %+v", len(got), len(tc.want), got)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("job %d: got %+v want %+v", i, got[i], tc.want[i])
				}
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
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "line 3") {
		t.Fatalf("error %q does not identify line 3", err)
	}
}
