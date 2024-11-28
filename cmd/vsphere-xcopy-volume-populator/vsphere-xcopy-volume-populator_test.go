package main_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/populator"
	storage_mocks "github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/populator/mocks"
	vmware_mocks "github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/vmware/mocks"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestPopulator(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	var vmwareClient = vmware_mocks.NewMockClient(mockCtrl)
	var storageClient = storage_mocks.NewMockStorageApi(mockCtrl)
	var underTest = populator.RemoteEsxcliPopulator{
		VSphereClient: vmwareClient,
		StorageApi:    storageClient,
	}

	var tests = []struct {
		name       string
		setup      func()
		sourceVMDK string
		targetPVC  string
		want       error
	}{
		{
			name:       "non valid vmdkPath source",
			sourceVMDK: "nonvalid.vmdk",
			targetPVC:  "pvc-12345",
			setup:      func() {},
			want:       fmt.Errorf("Invalid vmdkPath \"nonvalid.vmdk\", should be '[datastore] vmname/vmname.vmdk'"),
		},
		{
			name:       "fail resolution of the volumeHandle targetPVC",
			sourceVMDK: "[my-ds] my-vm/vmdisk.vmdk",
			targetPVC:  "pvc-12345",
			setup: func() {
				storageClient.EXPECT().ResolveVolumeHandleToLUN("pvc-12345").Return(populator.LUN{}, fmt.Errorf("")).Times(1)
			},
			want: fmt.Errorf(""),
		},
		{
			name:       "fail get current mapping of targetPVC",
			sourceVMDK: "[my-ds] my-vm/vmdisk.vmdk",
			targetPVC:  "pvc-12345",
			setup: func() {
				storageClient.EXPECT().ResolveVolumeHandleToLUN("pvc-12345").Return(populator.LUN{SerialNumber: "abc"}, nil)
				storageClient.EXPECT().CurrentMappedGroups(populator.LUN{SerialNumber: "abc"}).Return(nil, fmt.Errorf(""))
			},
			want: fmt.Errorf("failed to fetch the current initiator groups of the lun : %w", fmt.Errorf("")),
		},
		{
			name:       "fail get current mapping of targetPVC",
			sourceVMDK: "[my-ds] my-vm/vmdisk.vmdk",
			targetPVC:  "pvc-12345",
			setup: func() {
				storageClient.EXPECT().ResolveVolumeHandleToLUN("pvc-12345").Return(populator.LUN{SerialNumber: "abc"}, nil)
				storageClient.EXPECT().CurrentMappedGroups(populator.LUN{SerialNumber: "abc"}).Return(nil, fmt.Errorf(""))
			},
			want: fmt.Errorf("failed to fetch the current initiator groups of the lun : %w", fmt.Errorf("")),
		},

		{
			name:       "fail to locate an ESX",
			sourceVMDK: "[my-ds] my-vm/vmdisk.vmdk",
			targetPVC:  "pvc-12345",
			setup: func() {
				storageClient.EXPECT().ResolveVolumeHandleToLUN("pvc-12345").Return(populator.LUN{SerialNumber: "abc"}, nil).Times(1)
				storageClient.EXPECT().CurrentMappedGroups(populator.LUN{SerialNumber: "abc"})
				vmwareClient.EXPECT().GetEsxByVm(gomock.Any(), "my-vm").Return(nil, fmt.Errorf("")).Times(1)
			},
			want: fmt.Errorf(""),
		},

		{
			name: "working source and target",
			setup: func() {
				storageClient.EXPECT().ResolveVolumeHandleToLUN("pvc-12345").Return(populator.LUN{SerialNumber: "abc"}, nil)
				storageClient.EXPECT().CurrentMappedGroups(gomock.Any())
				storageClient.EXPECT().EnsureClonnerIgroup(gomock.Any(), gomock.Any())
				storageClient.EXPECT().Map("xcopy-esxs", gomock.Any())
				storageClient.EXPECT().UnMap(gomock.Any(), gomock.Any())
				vmwareClient.EXPECT().GetEsxByVm(context.Background(), gomock.Any())
				vmwareClient.EXPECT().RunEsxCommand(context.Background(), gomock.Any(),
					[]string{"iscsi", "adapter", "list"})
				vmwareClient.EXPECT().RunEsxCommand(context.Background(), gomock.Any(),
					[]string{"storage", "core", "adapter", "rescan", "-a", "1"})
				vmwareClient.EXPECT().RunEsxCommand(context.Background(), gomock.Any(),
					[]string{"vmkfstools", "clone", "-s", "[my-ds] my-vm/vmdisk.vmdk", "-t", "/vmfs/devices/disks/naa.616263"})
			},
			sourceVMDK: "[my-ds] my-vm/vmdisk.vmdk",
			targetPVC:  "pvc-12345",
			want:       nil,
		},
	}

	for _, tcase := range tests {
		t.Run(tcase.name, func(t *testing.T) {
			progressCh := make(chan int)
			quitCh := make(chan string)
			tcase.setup()
			result := underTest.Populate(tcase.sourceVMDK, tcase.targetPVC, progressCh, quitCh)
			assert.Equal(t, result, tcase.want)
		})
	}

}
