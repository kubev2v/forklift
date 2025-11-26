package populator

import (
	"context"
	"errors"
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
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockClient = vmware_mocks.NewMockClient(ctrl)
		host = &object.HostSystem{} // A dummy host system
		targetLUN = "naa.1234567890"
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
})

var _ = Describe("AdapterIdHandlerImpl", func() {
	var handler AdapterIdHandlerImpl

	BeforeEach(func() {
		handler = AdapterIdHandlerImpl{}
	})

	Describe("GetAdaptersID", func() {
		Context("when no adapters have been added", func() {
			It("should return an error", func() {
				ids, err := handler.GetAdaptersID()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("adapters ID are not set"))
				Expect(ids).To(BeNil())
			})
		})

		Context("when adapters have been added", func() {
			It("should return all added adapters", func() {
				handler.AddAdapterID("fc.2000000000000001:2100000000000001")
				handler.AddAdapterID("fc.2000000000000002:2100000000000002")
				handler.AddAdapterID("fc.2000000000000003:2100000000000003")

				ids, err := handler.GetAdaptersID()
				Expect(err).NotTo(HaveOccurred())
				Expect(ids).To(HaveLen(3))
				Expect(ids[0]).To(Equal("fc.2000000000000001:2100000000000001"))
				Expect(ids[1]).To(Equal("fc.2000000000000002:2100000000000002"))
				Expect(ids[2]).To(Equal("fc.2000000000000003:2100000000000003"))
			})

			It("should return consistent results on multiple calls", func() {
				handler.AddAdapterID("adapter1")
				handler.AddAdapterID("adapter2")

				ids1, err1 := handler.GetAdaptersID()
				Expect(err1).NotTo(HaveOccurred())

				ids2, err2 := handler.GetAdaptersID()
				Expect(err2).NotTo(HaveOccurred())

				Expect(ids1).To(Equal(ids2))
			})
		})
	})

	Describe("AddAdapterID", func() {
		Context("when adding a single adapter", func() {
			It("should store the adapter ID", func() {
				handler.AddAdapterID("fc.2000000000000001:2100000000000001")

				ids, err := handler.GetAdaptersID()
				Expect(err).NotTo(HaveOccurred())
				Expect(ids).To(HaveLen(1))
				Expect(ids[0]).To(Equal("fc.2000000000000001:2100000000000001"))
			})
		})

		Context("when adding multiple adapters", func() {
			It("should store all adapters in order", func() {
				adapters := []string{
					"fc.2000000000000001:2100000000000001",
					"fc.2000000000000002:2100000000000002",
					"fc.2000000000000003:2100000000000003",
				}

				for _, adapter := range adapters {
					handler.AddAdapterID(adapter)
				}

				ids, err := handler.GetAdaptersID()
				Expect(err).NotTo(HaveOccurred())
				Expect(ids).To(Equal(adapters))
			})
		})

		Context("when adding different adapter types", func() {
			It("should handle FC adapters", func() {
				handler.AddAdapterID("fc.2000000000000001:2100000000000001")

				ids, err := handler.GetAdaptersID()
				Expect(err).NotTo(HaveOccurred())
				Expect(ids).To(HaveLen(1))
				Expect(ids[0]).To(Equal("fc.2000000000000001:2100000000000001"))
			})

			It("should handle iSCSI adapters", func() {
				handler.AddAdapterID("iqn.1998-01.com.vmware:esxi-host1-12345678")

				ids, err := handler.GetAdaptersID()
				Expect(err).NotTo(HaveOccurred())
				Expect(ids).To(HaveLen(1))
				Expect(ids[0]).To(Equal("iqn.1998-01.com.vmware:esxi-host1-12345678"))
			})

			It("should handle NVMe adapters", func() {
				handler.AddAdapterID("nqn.2014-08.org.nvmexpress:uuid:12345678-1234-1234-1234-123456789abc")

				ids, err := handler.GetAdaptersID()
				Expect(err).NotTo(HaveOccurred())
				Expect(ids).To(HaveLen(1))
				Expect(ids[0]).To(Equal("nqn.2014-08.org.nvmexpress:uuid:12345678-1234-1234-1234-123456789abc"))
			})

			It("should handle mixed adapter types", func() {
				handler.AddAdapterID("fc.2000000000000001:2100000000000001")
				handler.AddAdapterID("iqn.1998-01.com.vmware:esxi-host1-12345678")
				handler.AddAdapterID("nqn.2014-08.org.nvmexpress:uuid:12345678-1234-1234-1234-123456789abc")

				ids, err := handler.GetAdaptersID()
				Expect(err).NotTo(HaveOccurred())
				Expect(ids).To(HaveLen(3))
				Expect(ids[0]).To(Equal("fc.2000000000000001:2100000000000001"))
				Expect(ids[1]).To(Equal("iqn.1998-01.com.vmware:esxi-host1-12345678"))
				Expect(ids[2]).To(Equal("nqn.2014-08.org.nvmexpress:uuid:12345678-1234-1234-1234-123456789abc"))
			})
		})

		Context("when adding duplicate adapters", func() {
			It("should allow duplicates (no deduplication)", func() {
				adapterID := "fc.2000000000000001:2100000000000001"
				handler.AddAdapterID(adapterID)
				handler.AddAdapterID(adapterID)
				handler.AddAdapterID(adapterID)

				ids, err := handler.GetAdaptersID()
				Expect(err).NotTo(HaveOccurred())
				Expect(ids).To(HaveLen(3))
				Expect(ids[0]).To(Equal(adapterID))
				Expect(ids[1]).To(Equal(adapterID))
				Expect(ids[2]).To(Equal(adapterID))
			})
		})
	})
})
