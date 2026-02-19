package main_test

import (
	"context"
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/populator"
	populator_mocks "github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/populator/mocks"
	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/version"
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
		storageClient *populator_mocks.MockStorageApi
		underTest     populator.RemoteEsxcliPopulator
		hostLocker    *populator_mocks.MockHostlocker
		dummyHost     *object.HostSystem
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		vmwareClient = vmware_mocks.NewMockClient(mockCtrl)
		storageClient = populator_mocks.NewMockStorageApi(mockCtrl)
		hostLocker = populator_mocks.NewMockHostlocker(mockCtrl)
		underTest = populator.RemoteEsxcliPopulator{
			VSphereClient: vmwareClient,
			StorageApi:    storageClient,
		}
		dummyHost = &object.HostSystem{
			Common: object.NewCommon(nil, types.ManagedObjectReference{ServerGUID: "HostSystem:host-1000"}),
		}
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	type testCase struct {
		name       string
		setup      func()
		sourceVmId string
		sourceVMDK string
		targetPVC  string
		want       error
	}

	DescribeTable("should handle various population scenarios",
		func(tc testCase) {
			progressCh := make(chan uint64)
			xcopyUsedCh := make(chan int)
			quitCh := make(chan error, 1)

			tc.setup()

			// Drain channels to prevent blocking
			go func() {
				for range progressCh {
				}
			}()
			go func() {
				for range xcopyUsedCh {

				}
			}()

			go func() {
				defer GinkgoRecover()
				underTest.Populate(tc.sourceVmId, tc.sourceVMDK, populator.PersistentVolume{Name: tc.targetPVC}, hostLocker, progressCh, xcopyUsedCh, quitCh)
			}()

			if tc.want != nil {
				if tc.want.Error() == "" {
					Eventually(quitCh, "20s").Should(Receive(HaveOccurred()))
				} else {
					var receivedErr error
					Eventually(quitCh, "20s").Should(Receive(&receivedErr))
					Expect(receivedErr.Error()).To(Equal(tc.want.Error()))
				}
			} else {
				Eventually(quitCh, "20s").Should(Receive(BeNil()))
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
				vmwareClient.EXPECT().GetEsxByVm(gomock.Any(), gomock.Any()).Return(dummyHost, nil)
				vmwareClient.EXPECT().RunEsxCommand(gomock.Any(), gomock.Any(), []string{"software", "vib", "get", "-n", "vmkfstools-wrapper"}).Return([]esx.Values{{"Version": {version.VibVersion}}}, nil)
				vmwareClient.EXPECT().RunEsxCommand(gomock.Any(), gomock.Any(), []string{"storage", "core", "adapter", "list"}).Return([]esx.Values{{"HBAName": {"vmhbatest"}, "UID": {"iqn.test"}, "LinkState": {"link-up"}, "Driver": {"iscsi"}}}, nil)
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
				vmwareClient.EXPECT().GetEsxByVm(gomock.Any(), gomock.Any()).Return(dummyHost, nil)
				vmwareClient.EXPECT().RunEsxCommand(gomock.Any(), gomock.Any(), []string{"software", "vib", "get", "-n", "vmkfstools-wrapper"}).Return([]esx.Values{{"Version": {version.VibVersion}}}, nil)
				vmwareClient.EXPECT().RunEsxCommand(gomock.Any(), gomock.Any(), []string{"storage", "core", "adapter", "list"}).Return([]esx.Values{{"HBAName": {"vmhbatest"}, "UID": {"iqn.test"}, "LinkState": {"link-up"}, "Driver": {"iscsi"}}}, nil)
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
				vmwareClient.EXPECT().GetEsxByVm(gomock.Any(), gomock.Any()).Return(dummyHost, nil)
				vmwareClient.EXPECT().RunEsxCommand(gomock.Any(), gomock.Any(), []string{"software", "vib", "get", "-n", "vmkfstools-wrapper"}).Return([]esx.Values{{"Version": {version.VibVersion}}}, nil)
				vmwareClient.EXPECT().RunEsxCommand(gomock.Any(), gomock.Any(), []string{"storage", "core", "adapter", "list"}).Return([]esx.Values{{"HBAName": {"vmhbatest"}, "UID": {"iqn.test"}, "LinkState": {"link-up"}, "Driver": {"iscsi"}}}, nil)
				storageClient.EXPECT().EnsureClonnerIgroup(gomock.Any(), gomock.Any()).Return(nil, nil)
				storageClient.EXPECT().ResolvePVToLUN(gomock.Any()).Return(populator.LUN{NAA: "naa.616263"}, nil)
				storageClient.EXPECT().CurrentMappedGroups(gomock.Any(), gomock.Any()).Return([]string{}, nil)
				storageClient.EXPECT().Map(gomock.Any(), gomock.Any(), nil).Return(populator.LUN{NAA: "naa.616263"}, nil)
				// Mock rescan device list call (happens inside hostLocker.WithLock) - returns "on" status
				vmwareClient.EXPECT().RunEsxCommand(gomock.Any(), gomock.Any(), []string{"storage", "core", "device", "list", "-d", "naa.616263"}).Return([]esx.Values{{"Status": {"on"}}}, nil).Times(1)
				vmwareClient.EXPECT().RunEsxCommand(gomock.Any(), gomock.Any(),
					[]string{"vmkfstools", "clone", "-s", "/vmfs/volumes/my-ds/my-vm/vmdisk.vmdk", "-t", "/vmfs/devices/disks/naa.616263"}).
					Return([]esx.Values{{"message": {`{"taskId": "1"}`}}}, nil)
				vmwareClient.EXPECT().RunEsxCommand(gomock.Any(), gomock.Any(), []string{"vmkfstools", "taskGet", "-i", "1"}).
					Return([]esx.Values{{"message": {`{"exitCode": "0"}`}}}, nil).Times(2)
				vmwareClient.EXPECT().RunEsxCommand(gomock.Any(), gomock.Any(), []string{"vmkfstools", "taskClean", "-i", "1"}).
					Return([]esx.Values{{"message": {`{"exitCode": "0"}`}}}, nil)
				vmwareClient.EXPECT().RunEsxCommand(gomock.Any(), gomock.Any(), []string{"storage", "core", "device", "set", "--state", "off", "-d", "naa.616263"}).Return(nil, nil)
				vmwareClient.EXPECT().RunEsxCommand(gomock.Any(), gomock.Any(), []string{"storage", "core", "device", "list", "-d", "naa.616263"}).Return([]esx.Values{map[string][]string{"Status": {"off"}}}, nil).AnyTimes()
				vmwareClient.EXPECT().RunEsxCommand(gomock.Any(), gomock.Any(), []string{"storage", "core", "device", "detached", "remove", "-d", "naa.616263"}).Return(nil, nil)
				vmwareClient.EXPECT().RunEsxCommand(gomock.Any(), gomock.Any(), []string{"storage", "core", "adapter", "rescan", "-t", "delete", "-A", "vmhbatest"}).Return(nil, nil)
				storageClient.EXPECT().UnMap(gomock.Any(), gomock.Any(), nil).Return(nil)
				// Mock hostLocker to actually execute the callback function
				hostLocker.EXPECT().WithLock(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, hostID string, work func(context.Context) error) error {
						return work(ctx)
					})
			},
			want: nil,
		}),
	)
})
