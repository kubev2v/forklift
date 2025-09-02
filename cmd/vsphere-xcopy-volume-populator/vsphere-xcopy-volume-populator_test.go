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
		sourceVmId string
		sourceVMDK string
		targetPVC  string
		want       error
	}{
		{
			name:       "non valid vmdkPath source",
			sourceVmId: "nonvalid.vmdk",
			sourceVMDK: "nonvalid.vmdk",
			targetPVC:  "pvc-12345",
			setup:      func() {},
			want:       fmt.Errorf("Invalid vmdkPath \"nonvalid.vmdk\", should be '[datastore] vmname/vmname.vmdk'"),
		},
		{
			name:       "fail resolution of the volumeHandle targetPVC",
			sourceVmId: "my-vm",
			sourceVMDK: "[my-ds] my-vm/vmdisk.vmdk",
			targetPVC:  "pvc-12345",
			setup: func() {
				vmwareClient.EXPECT().GetEsxByVm(context.Background(), gomock.Any()).Return(dummyHost, nil)
				vmwareClient.EXPECT().RunEsxCommand(context.Background(), gomock.Any(), []string{"software", "vib", "get", "-n", "vmkfstools-wrapper"}).Return([]esx.Values{{"Version": {populator.VibVersion}}}, nil)
				vmwareClient.EXPECT().RunEsxCommand(context.Background(), gomock.Any(), []string{"storage", "core", "adapter", "list"}).Return([]esx.Values{{"UID": {"iqn.test"}, "LinkState": {"link-up"}, "Driver": {"iscsi"}}}, nil)
				storageClient.EXPECT().EnsureClonnerIgroup(gomock.Any(), gomock.Any()).Return(nil, nil)
				storageClient.EXPECT().ResolvePVToLUN(populator.PersistentVolume{Name: "pvc-12345"}).Return(populator.LUN{}, fmt.Errorf("some error")).Times(1)
			},
			want: fmt.Errorf("some error"),
		}),
		Entry("fail get current mapping of targetPVC", testCase{
			sourceVmId: "my-vm",
			sourceVMDK: "[my-ds] my-vm/vmdisk.vmdk",
			targetPVC:  "pvc-12345",
			setup: func() {
				vmwareClient.EXPECT().GetEsxByVm(context.Background(), gomock.Any()).Return(dummyHost, nil)
				vmwareClient.EXPECT().RunEsxCommand(context.Background(), gomock.Any(), []string{"software", "vib", "get", "-n", "vmkfstools-wrapper"}).Return([]esx.Values{{"Version": {populator.VibVersion}}}, nil)
				vmwareClient.EXPECT().RunEsxCommand(context.Background(), gomock.Any(), []string{"storage", "core", "adapter", "list"}).Return([]esx.Values{{"UID": {"iqn.test"}, "LinkState": {"link-up"}, "Driver": {"iscsi"}}}, nil)
				storageClient.EXPECT().EnsureClonnerIgroup(gomock.Any(), gomock.Any()).Return(nil, nil)
				storageClient.EXPECT().ResolvePVToLUN(populator.PersistentVolume{Name: "pvc-12345"}).Return(populator.LUN{NAA: "616263"}, nil)
				storageClient.EXPECT().CurrentMappedGroups(populator.LUN{NAA: "616263"}, nil).Return(nil, fmt.Errorf("some error"))
			},
			want: fmt.Errorf("failed to fetch the current initiator groups of the lun : some error"),
		}),
		Entry("fail to locate an ESX", testCase{
			sourceVmId: "my-vm",
			sourceVMDK: "[my-ds] my-vm/vmdisk.vmdk",
			targetPVC:  "pvc-12345",
			setup: func() {
				vmwareClient.EXPECT().GetEsxByVm(gomock.Any(), "my-vm").Return(nil, fmt.Errorf("no host found")).Times(1)
			},
			want: fmt.Errorf("no host found"),
		}),
		Entry("working source and target", testCase{
			sourceVmId: "my-vm",
			sourceVMDK: "[my-ds] my-vm/vmdisk.vmdk",
			targetPVC:  "pvc-12345",
			setup: func() {
				vmwareClient.EXPECT().GetEsxByVm(context.Background(), gomock.Any()).Return(dummyHost, nil)
				vmwareClient.EXPECT().RunEsxCommand(context.Background(), gomock.Any(), []string{"software", "vib", "get", "-n", "vmkfstools-wrapper"}).Return([]esx.Values{{"Version": {populator.VibVersion}}}, nil)
				vmwareClient.EXPECT().RunEsxCommand(context.Background(), gomock.Any(), []string{"storage", "core", "adapter", "list"}).Return([]esx.Values{{"UID": {"iqn.test"}, "LinkState": {"link-up"}, "Driver": {"iscsi"}}}, nil)
				storageClient.EXPECT().EnsureClonnerIgroup(gomock.Any(), gomock.Any()).Return(nil, nil)
				storageClient.EXPECT().ResolvePVToLUN(gomock.Any()).Return(populator.LUN{NAA: "naa.616263"}, nil)
				storageClient.EXPECT().CurrentMappedGroups(gomock.Any(), gomock.Any()).Return([]string{}, nil)
				storageClient.EXPECT().Map("xcopy-esxs", gomock.Any(), nil).Return(populator.LUN{NAA: "naa.616263"}, nil)
				storageClient.EXPECT().UnMap(gomock.Any(), gomock.Any(), nil).AnyTimes()
				vmwareClient.EXPECT().RunEsxCommand(context.Background(), gomock.Any(), []string{"storage", "core", "device", "list", "-d", "/vmfs/devices/disks/naa.616263"}).Return(nil, nil)
				vmwareClient.EXPECT().RunEsxCommand(context.Background(), gomock.Any(),
					[]string{"iscsi", "adapter", "list"})
				storageClient.EXPECT().EnsureClonnerIgroup(gomock.Any(), gomock.Any())
				storageClient.EXPECT().ResolvePVToLUN("pvc-12345").Return(populator.LUN{}, fmt.Errorf("")).Times(1)
			},
			want: fmt.Errorf(""),
		},
		{
			name:       "fail get current mapping of targetPVC",
			sourceVmId: "my-vm",
			sourceVMDK: "[my-ds] my-vm/vmdisk.vmdk",
			targetPVC:  "pvc-12345",
			setup: func() {
				vmwareClient.EXPECT().GetEsxByVm(context.Background(), gomock.Any())
				vmwareClient.EXPECT().RunEsxCommand(context.Background(), gomock.Any(),
					[]string{"iscsi", "adapter", "list"})
				storageClient.EXPECT().EnsureClonnerIgroup(gomock.Any(), gomock.Any())
				storageClient.EXPECT().ResolvePVToLUN("pvc-12345").Return(populator.LUN{NAA: "616263"}, nil)
				storageClient.EXPECT().CurrentMappedGroups(populator.LUN{NAA: "616263"}, nil).Return(nil, fmt.Errorf(""))
			},
			want: fmt.Errorf("failed to fetch the current initiator groups of the lun : %w", fmt.Errorf("")),
		},
		{
			name:       "fail get current mapping of targetPVC",
			sourceVmId: "my-vm",
			sourceVMDK: "[my-ds] my-vm/vmdisk.vmdk",
			targetPVC:  "pvc-12345",
			setup: func() {
				vmwareClient.EXPECT().GetEsxByVm(context.Background(), gomock.Any())
				vmwareClient.EXPECT().RunEsxCommand(context.Background(), gomock.Any(),
					[]string{"iscsi", "adapter", "list"})
				storageClient.EXPECT().EnsureClonnerIgroup(gomock.Any(), gomock.Any())
				storageClient.EXPECT().ResolvePVToLUN("pvc-12345").Return(populator.LUN{NAA: "616263"}, nil)
				storageClient.EXPECT().CurrentMappedGroups(populator.LUN{NAA: "616263"}, nil).Return(nil, fmt.Errorf(""))
			},
			want: fmt.Errorf("failed to fetch the current initiator groups of the lun : %w", fmt.Errorf("")),
		},

		{
			name:       "fail to locate an ESX",
			sourceVmId: "my-vm",
			sourceVMDK: "[my-ds] my-vm/vmdisk.vmdk",
			targetPVC:  "pvc-12345",
			setup: func() {
				vmwareClient.EXPECT().GetEsxByVm(gomock.Any(), "my-vm").Return(nil, fmt.Errorf("")).Times(1)
				storageClient.EXPECT().ResolvePVToLUN("pvc-12345").Return(populator.LUN{NAA: "616263"}, nil).Times(1)
				storageClient.EXPECT().CurrentMappedGroups(populator.LUN{NAA: "616263"}, nil)
			},
			want: fmt.Errorf(""),
		},

		{
			name: "working source and target",
			setup: func() {
				vmwareClient.EXPECT().GetEsxByVm(context.Background(), gomock.Any())
				vmwareClient.EXPECT().RunEsxCommand(context.Background(), gomock.Any(),
					[]string{"iscsi", "adapter", "list"})
				storageClient.EXPECT().EnsureClonnerIgroup(gomock.Any(), gomock.Any())
				storageClient.EXPECT().Map("xcopy-esxs", gomock.Any(), nil).Return(populator.LUN{NAA: "616263"}, nil)
				storageClient.EXPECT().UnMap(gomock.Any(), gomock.Any(), nil)
				vmwareClient.EXPECT().RunEsxCommand(context.Background(), gomock.Any(),
					[]string{"storage", "core", "adapter", "rescan", "-a", "1"})
				vmwareClient.EXPECT().RunEsxCommand(context.Background(), gomock.Any(),
					[]string{"storage", "core", "device", "list", "-d", "naa.616263"})
				vmwareClient.EXPECT().RunEsxCommand(context.Background(), gomock.Any(),
					[]string{"vmkfstools", "clone", "-s", "/vmfs/volumes/my-ds/my-vm/vmdisk.vmdk", "-t", "/vmfs/devices/disks/naa.616263"})
			},
			sourceVmId: "my-vm",
			sourceVMDK: "[my-ds] my-vm/vmdisk.vmdk",
			targetPVC:  "pvc-12345",
			want:       nil,
		},
	}

	for _, tcase := range tests {
		t.Run(tcase.name, func(t *testing.T) {
			progressCh := make(chan uint)
			quitCh := make(chan error)
			tcase.setup()
			result := underTest.Populate(tcase.sourceVmId, tcase.sourceVMDK, populator.PersistentVolume{Name: tcase.targetPVC}, progressCh, quitCh)
			assert.Equal(t, result, tcase.want)
		})
	}

}
