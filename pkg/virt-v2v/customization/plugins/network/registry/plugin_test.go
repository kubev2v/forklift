package registry

import (
	"strings"
	"testing"

	"github.com/kubev2v/forklift/pkg/virt-v2v/config"
	"github.com/kubev2v/forklift/pkg/virt-v2v/customization/api"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestRegistryPlugin(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Network/Registry Plugin Suite")
}

var _ = Describe("Plugin", func() {
	var p *Plugin

	BeforeEach(func() {
		p = &Plugin{}
	})

	Describe("Applicable", func() {
		It("returns true for Windows + StaticIPs + registry flag", func() {
			ctx := &api.Context{
				Guest:  &api.GuestInfo{OS: api.GuestOS{Family: api.OSFamilyWindows}},
				Config: &config.AppConfig{StaticIPs: "aa:bb:cc:dd:ee:ff:ip:10.0.0.1", WindowsRegistryNetworkConfig: true},
			}
			Expect(p.Applicable(ctx)).To(BeTrue())
		})

		It("returns false for Linux", func() {
			ctx := &api.Context{
				Guest:  &api.GuestInfo{OS: api.GuestOS{Family: api.OSFamilyLinux}},
				Config: &config.AppConfig{StaticIPs: "aa:bb:cc:dd:ee:ff:ip:10.0.0.1", WindowsRegistryNetworkConfig: true},
			}
			Expect(p.Applicable(ctx)).To(BeFalse())
		})

		It("returns false when StaticIPs is empty", func() {
			ctx := &api.Context{
				Guest:  &api.GuestInfo{OS: api.GuestOS{Family: api.OSFamilyWindows}},
				Config: &config.AppConfig{WindowsRegistryNetworkConfig: true},
			}
			Expect(p.Applicable(ctx)).To(BeFalse())
		})

		It("returns false when registry flag is not set", func() {
			ctx := &api.Context{
				Guest:  &api.GuestInfo{OS: api.GuestOS{Family: api.OSFamilyWindows}},
				Config: &config.AppConfig{StaticIPs: "aa:bb:cc:dd:ee:ff:ip:10.0.0.1"},
			}
			Expect(p.Applicable(ctx)).To(BeFalse())
		})
	})

	Describe("Apply", func() {
		It("generates network config and duplicate routes scripts", func() {
			ctx := &api.Context{
				Guest:  &api.GuestInfo{OS: api.GuestOS{Family: api.OSFamilyWindows}},
				Config: &config.AppConfig{StaticIPs: "aa:bb:cc:dd:ee:ff:ip:10.0.0.1,10.0.0.254,24,8.8.8.8,", WindowsRegistryNetworkConfig: true},
			}
			actions, err := p.Apply(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(actions.Files).To(HaveLen(2))
			Expect(actions.Files[0].GuestPath).To(ContainSubstring("100_network_config.ps1"))
			Expect(actions.Files[1].GuestPath).To(ContainSubstring("120_remove_duplicate_routes.ps1"))
		})

		It("includes complementary IPs script when MultipleIpsPerNicName is set", func() {
			ctx := &api.Context{
				Guest: &api.GuestInfo{OS: api.GuestOS{Family: api.OSFamilyWindows}},
				Config: &config.AppConfig{
					StaticIPs:                    "aa:bb:cc:dd:ee:ff:ip:10.0.0.1,10.0.0.254,24,8.8.8.8,_aa:bb:cc:dd:ee:ff:ip:10.0.0.2,10.0.0.254,24,8.8.8.8,",
					WindowsRegistryNetworkConfig: true,
					MultipleIpsPerNicName:        "true",
				},
			}
			actions, err := p.Apply(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(actions.Files).To(HaveLen(3))
			Expect(actions.Files[2].GuestPath).To(ContainSubstring("110_complementary_ips.ps1"))
		})
	})

	Describe("injectStaticIPTemplate", func() {
		It("renders template with the input string", func() {
			rendered, err := injectStaticIPTemplate("aa:bb:cc:dd:ee:ff:ip:10.0.0.1,10.0.0.254,24,8.8.8.8,")
			Expect(err).NotTo(HaveOccurred())
			Expect(string(rendered)).To(ContainSubstring("aa:bb:cc:dd:ee:ff"))
			Expect(string(rendered)).To(ContainSubstring("10.0.0.1"))
		})
	})

	Describe("injectComplementaryIPTemplate", func() {
		It("renders complementary IPs for multi-IP MACs", func() {
			input := "aa:bb:cc:dd:ee:ff:ip:10.0.0.1,10.0.0.254,24,8.8.8.8,_aa:bb:cc:dd:ee:ff:ip:10.0.0.2,10.0.0.254,24,8.8.8.8,"
			rendered, err := injectComplementaryIPTemplate(input)
			Expect(err).NotTo(HaveOccurred())
			content := string(rendered)
			Expect(content).To(ContainSubstring("10.0.0.2"))
			Expect(strings.Contains(content, "aa-bb-cc-dd-ee-ff") || strings.Contains(content, "AA-BB-CC-DD-EE-FF")).To(BeTrue())
		})
	})
})
