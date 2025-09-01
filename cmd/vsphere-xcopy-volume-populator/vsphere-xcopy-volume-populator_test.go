package main_test

import (
	"context"
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/populator"
	storage_mocks "github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/populator/mocks"
	vmware_mocks "github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/vmware/mocks"
	"github.com/vmware/govmomi/cli/esx"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
	"go.uber.org/mock/gomock"
)

func TestPopulator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Populator Suite")
}

var _ = Describe("Populator", func() {
	var (
		mockCtrl      *gomock.Controller
		vmwareClient  *vmware_mocks.MockClient
		storageClient *storage_mocks.MockStorageApi
		underTest     populator.RemoteEsxcliPopulator
		dummyHost     *object.HostSystem
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		vmwareClient = vmware_mocks.NewMockClient(mockCtrl)
		storageClient = storage_mocks.NewMockStorageApi(mockCtrl)
		underTest = populator.RemoteEsxcliPopulator{
			VSphereClient: vmwareClient,
			StorageApi:    storageClient,
		}
		c := object.NewCommon(nil, types.ManagedObjectReference{
			Type:  "HostSystem",
			Value: "host-123",
		})
		dummyHost = &object.HostSystem{Common: c}
		dummyHost.InventoryPath = "host-123"
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	type testCase struct {
		name          string
		setup         func()
		sourceVmId    string
		sourceVMDK    string
		targetPVC     string
		migrationHost string
		want          error
	}

	DescribeTable("should handle various population scenarios",
		func(tc testCase) {
			progressCh := make(chan uint)
			quitCh := make(chan error, 1)

			tc.setup()

			go func() {
				defer GinkgoRecover()
				underTest.Populate(tc.sourceVmId, tc.migrationHost, tc.sourceVMDK, populator.PersistentVolume{Name: tc.targetPVC}, progressCh, quitCh)
			}()

			if tc.want != nil {
				if tc.want.Error() == "" {
					Eventually(quitCh, "10s").Should(Receive(HaveOccurred()))
				} else {
					var receivedErr error
					Eventually(quitCh, "10s").Should(Receive(&receivedErr))
					Expect(receivedErr.Error()).To(Equal(tc.want.Error()))
				}
			} else {
				Eventually(quitCh, "10s").Should(Receive(BeNil()))
			}
		},
		Entry("non valid vmdkPath source", testCase{
			sourceVmId: "nonvalid.vmdk",
			sourceVMDK: "nonvalid.vmdk",
			targetPVC:  "pvc-12345",
			setup:      func() {},
			want:       fmt.Errorf(`Invalid vmdkPath "nonvalid.vmdk", should be '[datastore] vmname/xyz.vmdk'`),
		}),
		Entry("fail resolution of the volumeHandle targetPVC", testCase{
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
				vmwareClient.EXPECT().GetEsxById(gomock.Any(), gomock.Any()).Times(0)
			},
			want: fmt.Errorf("no host found"),
		}),
		Entry("migrationHost - fail to locate an ESX by id", testCase{
			sourceVmId:    "my-vm",
			migrationHost: "host-123",
			sourceVMDK:    "[my-ds] my-vm/vmdisk.vmdk",
			targetPVC:     "pvc-12345",
			setup: func() {
				vmwareClient.EXPECT().GetEsxById(gomock.Any(), "host-123").Return(nil, fmt.Errorf("no host found")).Times(1)
				vmwareClient.EXPECT().GetEsxByVm(gomock.Any(), gomock.Any()).Times(0)
			},
			want: fmt.Errorf("no host found"),
		}),
		Entry("migrationHost - call on specific host instead of a vm host", testCase{
			sourceVmId:    "my-vm",
			migrationHost: "host-123",
			sourceVMDK:    "[my-ds] my-vm/vmdisk.vmdk",
			targetPVC:     "pvc-12345",
			setup: func() {
				vmwareClient.EXPECT().GetEsxByVm(gomock.Any(), gomock.Any()).Times(0)
				positivePathMockCalls(vmwareClient, storageClient, true)
			},
			want: nil,
		}),
		Entry("working source and target", testCase{
			sourceVmId: "my-vm",
			sourceVMDK: "[my-ds] my-vm/vmdisk.vmdk",
			targetPVC:  "pvc-12345",
			setup:      func() { positivePathMockCalls(vmwareClient, storageClient, false) },
			want:       nil,
		}),
	)
})

func positivePathMockCalls(vmwareClient *vmware_mocks.MockClient, storageClient *storage_mocks.MockStorageApi, useMigrationHost bool) {
	c := object.NewCommon(nil, types.ManagedObjectReference{
		Type:  "HostSystem",
		Value: "host-123",
	})
	dummyHost := &object.HostSystem{Common: c}

	if useMigrationHost {
		vmwareClient.EXPECT().GetEsxById(context.Background(), gomock.Any()).Return(dummyHost, nil)
	} else {
		vmwareClient.EXPECT().GetEsxByVm(context.Background(), gomock.Any()).Return(dummyHost, nil)
	}
	vmwareClient.EXPECT().RunEsxCommand(context.Background(), gomock.Any(), []string{"software", "vib", "get", "-n", "vmkfstools-wrapper"}).Return([]esx.Values{{"Version": {populator.VibVersion}}}, nil)
	vmwareClient.EXPECT().RunEsxCommand(context.Background(), gomock.Any(), []string{"storage", "core", "adapter", "list"}).Return([]esx.Values{{"UID": {"iqn.test"}, "LinkState": {"link-up"}, "Driver": {"iscsi"}}}, nil)
	storageClient.EXPECT().EnsureClonnerIgroup(gomock.Any(), gomock.Any()).Return(nil, nil)
	storageClient.EXPECT().ResolvePVToLUN(gomock.Any()).Return(populator.LUN{NAA: "naa.616263"}, nil)
	storageClient.EXPECT().CurrentMappedGroups(gomock.Any(), gomock.Any()).Return([]string{}, nil)
	storageClient.EXPECT().Map("xcopy-esxs", gomock.Any(), nil).Return(populator.LUN{NAA: "naa.616263"}, nil)
	storageClient.EXPECT().UnMap(gomock.Any(), gomock.Any(), nil).AnyTimes()
	vmwareClient.EXPECT().RunEsxCommand(context.Background(), gomock.Any(), []string{"storage", "core", "device", "list", "-d", "naa.616263"}).Return(nil, nil)
	vmwareClient.EXPECT().RunEsxCommand(context.Background(), gomock.Any(),
		[]string{"vmkfstools", "clone", "-s", "/vmfs/volumes/my-ds/my-vm/vmdisk.vmdk", "-t", "/vmfs/devices/disks/naa.616263"}).
		Return([]esx.Values{{"message": {`{"taskId": "1"}`}}}, nil)
	vmwareClient.EXPECT().RunEsxCommand(context.Background(), gomock.Any(), []string{"vmkfstools", "taskGet", "-i", "1"}).
		Return([]esx.Values{{"message": {`{"exitCode": "0"}`}}}, nil)
	vmwareClient.EXPECT().RunEsxCommand(context.Background(), gomock.Any(), []string{"vmkfstools", "taskClean", "-i", "1"}).Return(nil, nil)
	vmwareClient.EXPECT().RunEsxCommand(context.Background(), gomock.Any(), []string{"storage", "core", "adapter", "rescan", "-t", "delete", "-a", "1"}).Return(nil, nil)
}
