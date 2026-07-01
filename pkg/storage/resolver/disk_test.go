package resolver

import (
	"testing"
)

func TestDiskBackingClassify(t *testing.T) {
	tests := []struct {
		name    string
		backing DiskBacking
		want    DiskType
	}{
		{
			name:    "VVol-backed disk",
			backing: DiskBacking{VVolID: "vvol:12345678-1234-1234-1234-123456789abc"},
			want:    DiskTypeVVol,
		},
		{
			name:    "RDM-backed disk",
			backing: DiskBacking{IsRDM: true, DeviceName: "naa.600508b1001c1234567890abcdef1234"},
			want:    DiskTypeRDM,
		},
		{
			name:    "VMDK-backed disk (default)",
			backing: DiskBacking{DeviceName: "[datastore1] vm/vm.vmdk"},
			want:    DiskTypeVMDK,
		},
		{
			name:    "VVol takes precedence over RDM flag",
			backing: DiskBacking{VVolID: "vvol:some-id", IsRDM: true},
			want:    DiskTypeVVol,
		},
		{
			name:    "empty backing is VMDK",
			backing: DiskBacking{},
			want:    DiskTypeVMDK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectDiskType(&tt.backing)
			if got != tt.want {
				t.Errorf("DetectDiskType() = %q, want %q", got, tt.want)
			}
		})
	}
}
