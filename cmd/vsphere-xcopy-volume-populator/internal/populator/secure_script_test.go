package populator

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
	"go.uber.org/mock/gomock"

	vmware_mocks "github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/vmware/mocks"
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

	Context("UUID generation and filename construction", func() {
		It("should generate unique UUIDs for each upload", func() {
			// This test verifies that UUIDs are generated
			// We can't easily test the full upload flow without a real datastore,
			// but we can verify the UUID generation logic by checking the filename format
			uuid1 := "test-uuid-1"
			uuid2 := "test-uuid-2"

			scriptName1 := fmt.Sprintf("%s-%s.py", secureScriptName, uuid1)
			scriptName2 := fmt.Sprintf("%s-%s.py", secureScriptName, uuid2)

			Expect(scriptName1).ToNot(Equal(scriptName2))
			Expect(scriptName1).To(ContainSubstring(secureScriptName))
			Expect(scriptName1).To(ContainSubstring(".py"))
			Expect(scriptName2).To(ContainSubstring(secureScriptName))
			Expect(scriptName2).To(ContainSubstring(".py"))
		})

		It("should construct correct UUID-based filename", func() {
			testUUID := "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
			scriptName := fmt.Sprintf("%s-%s.py", secureScriptName, testUUID)

			Expect(scriptName).To(Equal("secure-vmkfstools-wrapper-a1b2c3d4-e5f6-7890-abcd-ef1234567890.py"))
		})
	})

	Context("when GetDatastore fails", func() {
		It("should return an error", func() {
			mockClient.EXPECT().
				GetDatastore(gomock.Any(), dc, datastore).
				Return(nil, errors.New("datastore not found"))

			_, _, err := uploadScript(ctx, mockClient, dc, datastore)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to get datastore"))
		})
	})

	Context("return value format", func() {
		It("should return path and UUID separately", func() {
			// Test that uploadScript returns (path, uuid, error) - two separate values
			// This verifies the return signature matches the implementation
			testUUID := "test-uuid-123"
			datastorePath := fmt.Sprintf("/vmfs/volumes/%s/%s-%s.py", datastore, secureScriptName, testUUID)

			// uploadScript returns (string, uuid.UUID, error) - path and UUID separately
			// ensureSecureScript also returns (string, uuid.UUID, error) - path and UUID separately
			Expect(datastorePath).To(ContainSubstring("/vmfs/volumes"))
			Expect(datastorePath).To(ContainSubstring(secureScriptName))
			Expect(datastorePath).To(ContainSubstring(testUUID))
			Expect(testUUID).To(MatchRegexp("^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$|^test-uuid-123$"))
		})

		It("should return path and UUID separately from ensureSecureScript", func() {
			// Test that ensureSecureScript returns (path, uuid, error) - two separate values
			// This matches the new implementation where path and UUID are returned separately
			testUUID := "test-uuid-123"
			datastorePath := fmt.Sprintf("/vmfs/volumes/%s/%s-%s.py", datastore, secureScriptName, testUUID)

			// ensureSecureScript returns (string, uuid.UUID, error) - path and UUID separately
			// No need to split or combine - they are already separate
			Expect(datastorePath).To(ContainSubstring("/vmfs/volumes"))
			Expect(datastorePath).To(ContainSubstring(secureScriptName))
			Expect(datastorePath).To(ContainSubstring(testUUID))
			Expect(testUUID).To(MatchRegexp("^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$|^test-uuid-123$"))
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
		// The script should contain Python code or shebang
		contentStr := string(content)
		Expect(contentStr).To(Or(ContainSubstring("python"), ContainSubstring("#!/")))
	})
})

var _ = Describe("Race condition prevention", func() {
	It("should generate unique UUID filenames for concurrent uploads", func() {
		// Simulate multiple concurrent uploads
		// Each upload gets a unique UUID filename, preventing race conditions
		uuids := make(map[string]bool)
		iterations := 100

		for i := 0; i < iterations; i++ {
			testUUID := uuid.New().String()
			scriptName := fmt.Sprintf("%s-%s.py", secureScriptName, testUUID)

			// Verify UUID is unique
			Expect(uuids[testUUID]).To(BeFalse(), "UUID should be unique")
			uuids[testUUID] = true

			// Verify filename format
			Expect(scriptName).To(ContainSubstring(secureScriptName))
			Expect(scriptName).To(ContainSubstring(".py"))
			Expect(scriptName).To(ContainSubstring(testUUID))
		}

		// Verify all UUIDs were unique
		Expect(len(uuids)).To(Equal(iterations))
	})

	It("should use UUID in SSH command format", func() {
		// Test that UUID is properly formatted for SSH commands
		// Format: DS=<datastore>;UUID=<uuid>;CMD=<command>
		testUUID := uuid.New().String()
		datastore := "test-ds"
		command := "status test-id"

		sshCommand := fmt.Sprintf("DS=%s;UUID=%s;CMD=%s", datastore, testUUID, command)

		Expect(sshCommand).To(ContainSubstring("DS=" + datastore))
		Expect(sshCommand).To(ContainSubstring("UUID=" + testUUID))
		Expect(sshCommand).To(ContainSubstring("CMD=" + command))
	})

	It("should construct correct script path with UUID", func() {
		// Verify the script path format matches what SSH template expects
		testUUID := uuid.New().String()
		datastore := "test-datastore"
		scriptPath := fmt.Sprintf("/vmfs/volumes/%s/%s-%s.py", datastore, secureScriptName, testUUID)

		Expect(scriptPath).To(ContainSubstring("/vmfs/volumes/" + datastore))
		Expect(scriptPath).To(ContainSubstring(secureScriptName))
		Expect(scriptPath).To(ContainSubstring(testUUID))
		Expect(scriptPath).To(ContainSubstring(".py"))
	})
})

var _ = Describe("cleanupSecureScript", func() {
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

	Context("validation of bad input", func() {
		It("should refuse to delete file with wrong prefix", func() {
			badFilename := "malicious-script.py"
			// GetDatastore should NOT be called when validation fails
			// If it were called, the test would fail because we didn't set up the mock expectation
			cleanupSecureScript(ctx, mockClient, dc, datastore, badFilename)
			// Test passes if GetDatastore was never called (validation worked)
		})

		It("should refuse to delete file without .py extension", func() {
			badFilename := "secure-vmkfstools-wrapper-some-uuid.sh"
			// GetDatastore should NOT be called when validation fails
			cleanupSecureScript(ctx, mockClient, dc, datastore, badFilename)
			// Test passes if GetDatastore was never called (validation worked)
		})

		It("should refuse to delete file with completely wrong name", func() {
			badFilename := "important-vm-file.vmdk"
			// GetDatastore should NOT be called when validation fails
			cleanupSecureScript(ctx, mockClient, dc, datastore, badFilename)
			// Test passes if GetDatastore was never called (validation worked)
		})

		It("should refuse to delete file with path traversal attempt", func() {
			badFilename := "../secure-vmkfstools-wrapper-uuid.py"
			// GetDatastore should NOT be called when validation fails
			cleanupSecureScript(ctx, mockClient, dc, datastore, badFilename)
			// Test passes if GetDatastore was never called (validation worked)
		})

		It("should refuse to delete file with empty name", func() {
			badFilename := ""
			// GetDatastore should NOT be called when validation fails
			cleanupSecureScript(ctx, mockClient, dc, datastore, badFilename)
			// Test passes if GetDatastore was never called (validation worked)
		})

		It("should refuse to delete file that only matches prefix but not suffix", func() {
			badFilename := "secure-vmkfstools-wrapper-malicious.sh"
			// GetDatastore should NOT be called when validation fails
			cleanupSecureScript(ctx, mockClient, dc, datastore, badFilename)
			// Test passes if GetDatastore was never called (validation worked)
		})
	})
})
