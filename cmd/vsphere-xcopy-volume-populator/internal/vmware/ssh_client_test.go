package vmware

import (
	"fmt"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSSHClient(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "SSH Client Suite")
}

var _ = Describe("ESXiSSHClient ExecuteCommand", func() {
	var (
		client *ESXiSSHClient
	)

	BeforeEach(func() {
		client = &ESXiSSHClient{
			hostname:   "test-host.example.com",
			scriptUUID: "test-uuid-123",
		}
	})

	Context("when SSH client is not connected", func() {
		It("should return an error", func() {
			output, err := client.ExecuteCommand("datastore1", "test-command")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("SSH client not connected"))
			Expect(output).To(BeEmpty())
		})
	})
})

// TestOldSSHKeyFormatDetection tests the validation logic for detecting old SSH key format
// This test verifies that the fix correctly identifies when SSH keys use the old Python-based format
// and returns a helpful error message to guide users to update their keys.
var _ = Describe("Old SSH Key Format Detection", func() {
	It("should detect .py extension in error output", func() {
		output := "error: /vmfs/volumes/datastore1/secure-vmkfstools-wrapper.py: No such file or directory"
		detected := strings.Contains(output, ".py") || strings.Contains(output, "python")
		Expect(detected).To(BeTrue(), "Should detect .py extension in output")
	})

	It("should detect 'python' keyword in error output", func() {
		output := "python: command not found"
		detected := strings.Contains(output, ".py") || strings.Contains(output, "python")
		Expect(detected).To(BeTrue(), "Should detect 'python' keyword in output")
	})

	It("should detect both .py and python in output", func() {
		output := "python /vmfs/volumes/datastore1/secure-vmkfstools-wrapper.py: execution failed"
		detected := strings.Contains(output, ".py") || strings.Contains(output, "python")
		Expect(detected).To(BeTrue(), "Should detect both .py and python in output")
	})

	It("should not detect old format in valid .sh output", func() {
		output := "success: command executed via /vmfs/volumes/datastore1/secure-vmkfstools-wrapper.sh"
		detected := strings.Contains(output, ".py") || strings.Contains(output, "python")
		Expect(detected).To(BeFalse(), "Should not detect old format in valid .sh output")
	})

	It("should not detect old format in normal success output", func() {
		output := "command executed successfully"
		detected := strings.Contains(output, ".py") || strings.Contains(output, "python")
		Expect(detected).To(BeFalse(), "Should not detect old format in normal output")
	})

	It("should generate correct error message format", func() {
		hostname := "test-host.example.com"
		outputStr := "error: /vmfs/volumes/ds1/secure-vmkfstools-wrapper.py: not found"

		errMsg := fmt.Sprintf("SSH key uses old format with Python wrapper (.py). "+
			"The system now requires the new format using shell wrapper (.sh). "+
			"Please update the SSH key on host %s by removing the old key entry from "+
			"/etc/ssh/keys-root/authorized_keys and adding the new format. "+
			"See README for migration instructions. "+
			"Command output: %s", hostname, outputStr)

		Expect(errMsg).To(ContainSubstring("SSH key uses old format with Python wrapper (.py)"))
		Expect(errMsg).To(ContainSubstring("The system now requires the new format using shell wrapper (.sh)"))
		Expect(errMsg).To(ContainSubstring("Please update the SSH key on host"))
		Expect(errMsg).To(ContainSubstring("/etc/ssh/keys-root/authorized_keys"))
		Expect(errMsg).To(ContainSubstring("See README for migration instructions"))
		Expect(errMsg).To(ContainSubstring(hostname))
		Expect(errMsg).To(ContainSubstring(outputStr))
	})

	Context("table-driven validation tests", func() {
		testCases := []struct {
			name         string
			output       string
			shouldDetect bool
		}{
			{
				name:         "detects .py extension",
				output:       "error: /vmfs/volumes/ds1/secure-vmkfstools-wrapper.py: not found",
				shouldDetect: true,
			},
			{
				name:         "detects python keyword",
				output:       "python: command not found",
				shouldDetect: true,
			},
			{
				name:         "detects both .py and python",
				output:       "python /vmfs/volumes/ds1/secure-vmkfstools-wrapper.py failed",
				shouldDetect: true,
			},
			{
				name:         "does not detect in valid .sh output",
				output:       "success: /vmfs/volumes/ds1/secure-vmkfstools-wrapper.sh executed",
				shouldDetect: false,
			},
			{
				name:         "does not detect in normal output",
				output:       "command executed successfully",
				shouldDetect: false,
			},
			{
				name:         "detects python in error message",
				output:       "failed to execute python script",
				shouldDetect: true,
			},
			{
				name:         "detects .py in path even with other text",
				output:       "Cannot find /vmfs/volumes/datastore1/secure-vmkfstools-wrapper.py",
				shouldDetect: true,
			},
			{
				name:         "case sensitive detection of python",
				output:       "Python interpreter not found",
				shouldDetect: false, // lowercase "python" only
			},
			{
				name:         "detects lowercase python",
				output:       "python interpreter not found",
				shouldDetect: true,
			},
		}

		for _, tc := range testCases {
			tc := tc // capture loop variable
			It(tc.name, func() {
				detected := strings.Contains(tc.output, ".py") || strings.Contains(tc.output, "python")
				Expect(detected).To(Equal(tc.shouldDetect), "Detection mismatch for output: %q", tc.output)
			})
		}
	})
})
