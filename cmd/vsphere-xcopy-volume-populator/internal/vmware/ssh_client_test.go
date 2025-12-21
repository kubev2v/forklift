package vmware

import (
	"context"
	"fmt"
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
			hostname: "test-host.example.com",
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

// stubSSHClient is a test implementation of SSHClient for testing CheckScriptVersion
type stubSSHClient struct {
	executeResponse string
	executeError    error
}

func (s *stubSSHClient) Connect(ctx context.Context, hostname, username string, privateKey []byte) error {
	return nil
}

func (s *stubSSHClient) ExecuteCommand(datastore, sshCommand string, args ...string) (string, error) {
	return s.executeResponse, s.executeError
}

func (s *stubSSHClient) Close() error {
	return nil
}

var _ = Describe("CheckScriptVersion", func() {
	var (
		client    *stubSSHClient
		datastore string
		publicKey []byte
	)

	BeforeEach(func() {
		client = &stubSSHClient{}
		datastore = "test-datastore"
		publicKey = []byte("ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQ... test@example.com")
	})

	Context("when script version matches embedded version", func() {
		It("should succeed", func() {
			client.executeResponse = `<?xml version="1.0" ?>
<output xmlns="http://www.vmware.com/Products/ESX/5.0/esxcli/">
    <structure typeName="result">
        <field name="status"><string>0</string></field>
        <field name="message"><string>{"version": "0.3.0"}</string></field>
    </structure>
</output>`

			err := CheckScriptVersion(client, datastore, "0.3.0", publicKey)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("when script version is newer than embedded version", func() {
		It("should succeed", func() {
			client.executeResponse = `<?xml version="1.0" ?>
<output xmlns="http://www.vmware.com/Products/ESX/5.0/esxcli/">
    <structure typeName="result">
        <field name="status"><string>0</string></field>
        <field name="message"><string>{"version": "0.4.0"}</string></field>
    </structure>
</output>`

			err := CheckScriptVersion(client, datastore, "0.3.0", publicKey)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("when script version is older than embedded version", func() {
		It("should return error indicating old SSH key format", func() {
			client.executeResponse = `<?xml version="1.0" ?>
<output xmlns="http://www.vmware.com/Products/ESX/5.0/esxcli/">
    <structure typeName="result">
        <field name="status"><string>0</string></field>
        <field name="message"><string>{"version": "0.2.0"}</string></field>
    </structure>
</output>`

			err := CheckScriptVersion(client, datastore, "0.3.0", publicKey)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("version mismatch"))
			Expect(err.Error()).To(ContainSubstring("uploaded 0.3.0 but SSH returned 0.2.0"))
			Expect(err.Error()).To(ContainSubstring("old SSH key format detected"))
		})
	})

	Context("when ExecuteCommand fails", func() {
		It("should return error indicating old script format", func() {
			client.executeError = fmt.Errorf("command failed: file not found")

			err := CheckScriptVersion(client, datastore, "0.3.0", publicKey)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("old script format detected"))
			Expect(err.Error()).To(ContainSubstring("Python-based"))
		})
	})

	Context("when XML response is invalid", func() {
		It("should return parsing error", func() {
			client.executeResponse = "not valid XML"

			err := CheckScriptVersion(client, datastore, "0.3.0", publicKey)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to parse version response"))
		})
	})

	Context("when status is non-zero", func() {
		It("should return error", func() {
			client.executeResponse = `<?xml version="1.0" ?>
<output xmlns="http://www.vmware.com/Products/ESX/5.0/esxcli/">
    <structure typeName="result">
        <field name="status"><string>1</string></field>
        <field name="message"><string>command failed</string></field>
    </structure>
</output>`

			err := CheckScriptVersion(client, datastore, "0.3.0", publicKey)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("version command failed"))
		})
	})

	Context("when JSON in message is invalid", func() {
		It("should return JSON parsing error", func() {
			client.executeResponse = `<?xml version="1.0" ?>
<output xmlns="http://www.vmware.com/Products/ESX/5.0/esxcli/">
    <structure typeName="result">
        <field name="status"><string>0</string></field>
        <field name="message"><string>not valid json</string></field>
    </structure>
</output>`

			err := CheckScriptVersion(client, datastore, "0.3.0", publicKey)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to parse version JSON"))
		})
	})

	Context("version comparison edge cases", func() {
		DescribeTable("version comparisons",
			func(scriptVersion, embeddedVersion string, shouldSucceed bool) {
				client.executeResponse = fmt.Sprintf(`<?xml version="1.0" ?>
<output xmlns="http://www.vmware.com/Products/ESX/5.0/esxcli/">
    <structure typeName="result">
        <field name="status"><string>0</string></field>
        <field name="message"><string>{"version": "%s"}</string></field>
    </structure>
</output>`, scriptVersion)

				err := CheckScriptVersion(client, datastore, embeddedVersion, publicKey)
				if shouldSucceed {
					Expect(err).ToNot(HaveOccurred(), "Expected version %s >= %s to succeed", scriptVersion, embeddedVersion)
				} else {
					Expect(err).To(HaveOccurred(), "Expected version %s < %s to fail", scriptVersion, embeddedVersion)
				}
			},
			Entry("1.0 vs 1.0.0 should be equal", "1.0", "1.0.0", true),
			Entry("2.0.0 vs 1.9.9 - script is newer", "2.0.0", "1.9.9", true),
			Entry("1.9.9 vs 2.0.0 - script is older", "1.9.9", "2.0.0", false),
			Entry("0.10.0 vs 0.9.0 - script is newer", "0.10.0", "0.9.0", true),
			Entry("0.3.0 vs 0.3.0 - exact match", "0.3.0", "0.3.0", true),
		)
	})

	Context("when version format is invalid", func() {
		It("should return error for invalid script version", func() {
			client.executeResponse = `<?xml version="1.0" ?>
<output xmlns="http://www.vmware.com/Products/ESX/5.0/esxcli/">
    <structure typeName="result">
        <field name="status"><string>0</string></field>
        <field name="message"><string>{"version": "not-a-version"}</string></field>
    </structure>
</output>`

			err := CheckScriptVersion(client, datastore, "0.3.0", publicKey)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid script version format"))
		})

		It("should return error for invalid embedded version", func() {
			client.executeResponse = `<?xml version="1.0" ?>
<output xmlns="http://www.vmware.com/Products/ESX/5.0/esxcli/">
    <structure typeName="result">
        <field name="status"><string>0</string></field>
        <field name="message"><string>{"version": "0.3.0"}</string></field>
    </structure>
</output>`

			err := CheckScriptVersion(client, datastore, "invalid-version", publicKey)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid embedded version format"))
		})
	})
})
