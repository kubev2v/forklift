package populator

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/vmware/govmomi/object"
	"go.uber.org/mock/gomock"

	vmware_mocks "github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/vmware/mocks"
)

func TestRemoteEsxcli(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Remote ESXCLI Suite")
}

var _ = Describe("rescan", func() {
	var (
		ctrl       *gomock.Controller
		mockClient *vmware_mocks.MockClient
		host       *object.HostSystem
		targetLUN  string
		preferFCAdapters bool
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockClient = vmware_mocks.NewMockClient(ctrl)
		host = &object.HostSystem{} // A dummy host system
		targetLUN = "naa.1234567890"
		preferFCAdapters = false
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Context("when the device is found on the first attempt", func() {
		It("should return nil", func() {
			listCmd := []string{"storage", "core", "device", "list", "-d", targetLUN}
			mockClient.EXPECT().RunEsxCommand(gomock.Any(), host, gomock.Eq(listCmd)).Return(nil, nil)

			err := rescan(context.Background(), mockClient, host, targetLUN)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("when the device is found after a rescan", func() {
		It("should return nil", func() {
			listCmd := []string{"storage", "core", "device", "list", "-d", targetLUN}
			rescanCmd := []string{"storage", "core", "adapter", "rescan", "-t", "add", "-a", "1"}

			gomock.InOrder(
				mockClient.EXPECT().RunEsxCommand(gomock.Any(), host, gomock.Eq(listCmd)).Return(nil, errors.New("device not found")),
				mockClient.EXPECT().RunEsxCommand(gomock.Any(), host, gomock.Eq(rescanCmd)).Return(nil, nil),
				mockClient.EXPECT().RunEsxCommand(gomock.Any(), host, gomock.Eq(listCmd)).Return(nil, nil),
			)

			err := rescan(context.Background(), mockClient, host, targetLUN)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("when the device is never found", func() {
		It("should return an error", func() {
			listCmd := []string{"storage", "core", "device", "list", "-d", targetLUN}
			rescanCmd := []string{"storage", "core", "adapter", "rescan", "-t", "add", "-a", "1"}

			mockClient.EXPECT().RunEsxCommand(gomock.Any(), host, gomock.Eq(listCmd)).Return(nil, errors.New("device not found")).Times(rescanRetries + 1)
			mockClient.EXPECT().RunEsxCommand(gomock.Any(), host, gomock.Eq(rescanCmd)).Return(nil, nil).Times(rescanRetries)

			err := rescan(context.Background(), mockClient, host, targetLUN)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to find device"))
		})
	})

	Context("when rescan command always fails", func() {
		It("should retry and eventually fail if device not found", func() {
			listCmd := []string{"storage", "core", "device", "list", "-d", targetLUN}
			rescanCmd := []string{"storage", "core", "adapter", "rescan", "-t", "add", "-a", "1"}

			mockClient.EXPECT().RunEsxCommand(gomock.Any(), host, gomock.Eq(listCmd)).Return(nil, errors.New("device not found")).Times(rescanRetries + 1)
			mockClient.EXPECT().RunEsxCommand(gomock.Any(), host, gomock.Eq(rescanCmd)).Return(nil, errors.New("rescan failed")).Times(rescanRetries)

			err := rescan(context.Background(), mockClient, host, targetLUN)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to find device"))
		})
	})

	Context("when rescan command fails", func() {
		It("should retry and eventually succeed if device found", func() {
			listCmd := []string{"storage", "core", "device", "list", "-d", targetLUN}
			rescanCmd := []string{"storage", "core", "adapter", "rescan", "-t", "add", "-a", "1"}

			mockClient.EXPECT().RunEsxCommand(gomock.Any(), host, gomock.Eq(listCmd)).Return(nil, errors.New("device not found")).Times(3)
			mockClient.EXPECT().RunEsxCommand(gomock.Any(), host, gomock.Eq(listCmd)).Return(nil, nil).Times(1)
			mockClient.EXPECT().RunEsxCommand(gomock.Any(), host, gomock.Eq(rescanCmd)).Return(nil, errors.New("rescan failed")).Times(2)
			mockClient.EXPECT().RunEsxCommand(gomock.Any(), host, gomock.Eq(rescanCmd)).Return(nil, nil).Times(1)

			err := rescan(context.Background(), mockClient, host, targetLUN)
			Expect(err).ToNot(HaveOccurred())
		})
		It("should retry even when scan fails and eventually succeed if device found", func() {
			listCmd := []string{"storage", "core", "device", "list", "-d", targetLUN}
			rescanCmd := []string{"storage", "core", "adapter", "rescan", "-t", "add", "-a", "1"}

			mockClient.EXPECT().RunEsxCommand(gomock.Any(), host, gomock.Eq(listCmd)).Return(nil, errors.New("device not found")).Times(3)
			mockClient.EXPECT().RunEsxCommand(gomock.Any(), host, gomock.Eq(listCmd)).Return(nil, nil).Times(1)
			mockClient.EXPECT().RunEsxCommand(gomock.Any(), host, gomock.Eq(rescanCmd)).Return(nil, errors.New("rescan failed")).Times(3)

			err := rescan(context.Background(), mockClient, host, targetLUN)
			Expect(err).ToNot(HaveOccurred())
		})
	})
	Context("when preferFCAdapters is true", func() {
		It("should sort the HBA UIDs in ascending order", func() {
			hbaUIDs := []string{"fc.1234567890", "iqn.1234567890", "iqn.234567891"}
			slices.SortFunc(hbaUIDs, func(a, b string) int {
				if preferFCAdapters {
					return strings.Compare(b, a)
				}
				return strings.Compare(a, b)
			})
			Expect(hbaUIDs).To(Equal([]string{"fc.1234567890", "iqn.1234567890", "iqn.234567891"}))
		})
	})
})
