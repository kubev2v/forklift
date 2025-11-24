package populator_test

import (
	"testing"

	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/populator"
	populator_mocks "github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/populator/populator_mocks"
	"go.uber.org/mock/gomock"
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
			name:           "Invalid VMDK Path - missing ']'",
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

func TestPopulate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPopulator := populator_mocks.NewMockPopulator(ctrl)
	pv := populator.PersistentVolume{
		Name:         "test-pv",
		VolumeHandle: "test-handle",
	}
	progress := make(chan uint)
	quit := make(chan error)

	mockPopulator.EXPECT().Populate("vm-1", "source.vmdk", pv, progress, quit).Return(nil)

	err := mockPopulator.Populate("vm-1", "source.vmdk", pv, progress, quit)
	if err != nil {
		t.Errorf("Populate() error = %v, wantErr %v", err, false)
	}
}
