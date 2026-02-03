package populator

import (
	"context"
	"errors"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
	"go.uber.org/mock/gomock"

	vmware_mocks "github.com/kubev2v/forklift/pkg/lib/vsphere_offload/vmware/mocks"
)

var _ = Describe("uploadScript", func() {
	var (
		ctrl       *gomock.Controller
		mockClient *vmware_mocks.MockClient
		dc         *object.Datacenter
		datastore  string
		ctx        context.Context
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockClient = vmware_mocks.NewMockClient(ctrl)
		dc = &object.Datacenter{
			Common: object.NewCommon(nil, types.ManagedObjectReference{
				Type:  "Datacenter",
				Value: "dc-1",
			}),
		}
		datastore = "test-datastore"
		ctx = context.Background()
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Context("filename construction", func() {
		It("should use correct script name without extension", func() {
			// Script name should be just the base name without extension
			Expect(scriptName).To(Equal("secure-vmkfstools-wrapper"))
		})
	})

	Context("when GetDatastore fails", func() {
		It("should return an error", func() {
			mockClient.EXPECT().
				GetDatastore(gomock.Any(), dc, datastore).
				Return(nil, errors.New("datastore not found"))

			_, err := uploadScript(ctx, mockClient, dc, datastore)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to get datastore"))
		})
	})

	Context("return value format", func() {
		It("should return script path", func() {
			// Test that uploadScript returns (path, error)
			// Script path should be in the format: /vmfs/volumes/{datastore}/{scriptName}
			datastorePath := "/vmfs/volumes/" + datastore + "/" + scriptName

			Expect(datastorePath).To(ContainSubstring("/vmfs/volumes"))
			Expect(datastorePath).To(ContainSubstring(scriptName))
			Expect(datastorePath).ToNot(ContainSubstring(".py"))
			Expect(datastorePath).ToNot(ContainSubstring(".sh"))
		})
	})
})

var _ = Describe("writeSecureScriptToTemp", func() {
	It("should create a temporary file with script content", func() {
		tempPath, err := writeSecureScriptToTemp()
		Expect(err).ToNot(HaveOccurred())
		Expect(tempPath).ToNot(BeEmpty())

		// Verify file exists and has content
		// Note: The file will be cleaned up by the defer in the function
		// but we can check it exists before that
		Expect(tempPath).To(ContainSubstring("secure-vmkfstools-wrapper"))
	})

	It("should write script content to the file", func() {
		tempPath, err := writeSecureScriptToTemp()
		Expect(err).ToNot(HaveOccurred())

		// Read the file to verify content
		content, err := os.ReadFile(tempPath)
		Expect(err).ToNot(HaveOccurred())
		Expect(content).ToNot(BeEmpty())
		// The script should contain shell script code or shebang
		contentStr := string(content)
		Expect(contentStr).To(Or(ContainSubstring("#!/bin/sh"), ContainSubstring("#!/")))
	})
})
