package populator_test

import (
	"testing"

	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/populator"
)

func TestVMDisk_Path(t *testing.T) {
	tests := []struct {
		name     string
		disk     populator.VMDisk
		expected string
	}{
		{
			name: "Standard VMDisk Path",
			disk: populator.VMDisk{
				Datastore: "mydatastore",
				VmHomeDir: "vm-1",
				VmdkFile:  "disk-1.vmdk",
			},
			expected: "/vmfs/volumes/mydatastore/vm-1/disk-1.vmdk",
		},
		{
			name:     "Empty VMDisk fields",
			disk:     populator.VMDisk{},
			expected: "/vmfs/volumes///",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.disk.Path()
			if got != tt.expected {
				t.Errorf("\ngot  %q\nwant %q", got, tt.expected)
			}
		})
	}
}

func TestParseVmdkPath(t *testing.T) {
	tests := []struct {
		name           string
		vmdkPath       string
		expectedVMDisk populator.VMDisk
		expectError    bool
	}{
		{
			name:           "Valid VMDK Path",
			vmdkPath:       "[mydatastore] vm-1/disk-1.vmdk",
			expectedVMDisk: populator.VMDisk{VmHomeDir: "vm-1", Datastore: "mydatastore", VmdkFile: "disk-1.vmdk"},
			expectError:    false,
		},
		{
			name:           "Valid VMDK Path with spaces",
			vmdkPath:       "[my datastore] my vm/my vm-disk-1.vmdk",
			expectedVMDisk: populator.VMDisk{VmHomeDir: "my vm", Datastore: "my datastore", VmdkFile: "my vm-disk-1.vmdk"},
			expectError:    false,
		},
		{
			name:           "Invalid VMDK Path - missing ']' ",
			vmdkPath:       "[mydatastore myvm/myvm.vmdk",
			expectedVMDisk: populator.VMDisk{},
			expectError:    true,
		},
		{
			name:           "Invalid VMDK Path - missing '/' ",
			vmdkPath:       "[mydatastore] myvm_myvm.vmdk",
			expectedVMDisk: populator.VMDisk{},
			expectError:    true,
		},
		{
			name:           "Empty VMDK Path",
			vmdkPath:       "",
			expectedVMDisk: populator.VMDisk{},
			expectError:    true,
		},
		{
			name:           "VMDK Path with only datastore",
			vmdkPath:       "[mydatastore]",
			expectedVMDisk: populator.VMDisk{},
			expectError:    true,
		},
		{
			name:           "VMDK Path with multiple slashes in path",
			vmdkPath:       "[mydatastore] myvm/subdir/myvm.vmdk",
			expectedVMDisk: populator.VMDisk{},
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := populator.ParseVmdkPath(tt.vmdkPath)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected an error, but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if got != tt.expectedVMDisk {
					t.Errorf("\ngot  %+v,\nwant %+v", got, tt.expectedVMDisk)
				}
			}
		})
	}
}
