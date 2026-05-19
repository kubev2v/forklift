package netsh

import (
	"testing"

	"github.com/kubev2v/forklift/pkg/virt-v2v/config"
	"github.com/kubev2v/forklift/pkg/virt-v2v/customization/api"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestNetshPlugin(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Network/Netsh Plugin Suite")
}

var _ = Describe("Plugin", func() {
	var p *Plugin

	BeforeEach(func() {
		p = &Plugin{}
	})

	Describe("Applicable", func() {
		It("returns true for Windows + StaticIPs + legacy drivers + no registry flag", func() {
			ctx := &api.Context{
				Guest:  &api.GuestInfo{OS: api.GuestOS{Family: api.OSFamilyWindows}},
				Config: &config.AppConfig{StaticIPs: "aa:bb:cc:dd:ee:ff:ip:10.0.0.1", VirtIoWinLegacyDrivers: "/mnt/virtio.iso"},
			}
			Expect(p.Applicable(ctx)).To(BeTrue())
		})

		It("returns false for Linux", func() {
			ctx := &api.Context{
				Guest:  &api.GuestInfo{OS: api.GuestOS{Family: api.OSFamilyLinux}},
				Config: &config.AppConfig{StaticIPs: "aa:bb:cc:dd:ee:ff:ip:10.0.0.1", VirtIoWinLegacyDrivers: "/mnt/virtio.iso"},
			}
			Expect(p.Applicable(ctx)).To(BeFalse())
		})

		It("returns false when StaticIPs is empty", func() {
			ctx := &api.Context{
				Guest:  &api.GuestInfo{OS: api.GuestOS{Family: api.OSFamilyWindows}},
				Config: &config.AppConfig{VirtIoWinLegacyDrivers: "/mnt/virtio.iso"},
			}
			Expect(p.Applicable(ctx)).To(BeFalse())
		})

		It("returns false when registry flag is set", func() {
			ctx := &api.Context{
				Guest: &api.GuestInfo{OS: api.GuestOS{Family: api.OSFamilyWindows}},
				Config: &config.AppConfig{
					StaticIPs:                    "aa:bb:cc:dd:ee:ff:ip:10.0.0.1",
					VirtIoWinLegacyDrivers:       "/mnt/virtio.iso",
					WindowsRegistryNetworkConfig: true,
				},
			}
			Expect(p.Applicable(ctx)).To(BeFalse())
		})

		It("returns false when no legacy drivers", func() {
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
				Config: &config.AppConfig{StaticIPs: "aa:bb:cc:dd:ee:ff:ip:10.0.0.1,10.0.0.254,24,8.8.8.8,", VirtIoWinLegacyDrivers: "/mnt/virtio.iso"},
			}
			actions, err := p.Apply(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(actions.Files).To(HaveLen(2))
			Expect(actions.Files[0].GuestPath).To(ContainSubstring("100_network_config.ps1"))
			Expect(actions.Files[1].GuestPath).To(ContainSubstring("120_remove_duplicate_routes.ps1"))
		})
	})

	Describe("renderTemplate", func() {
		It("renders template with the input string", func() {
			rendered, err := renderTemplate("100_network_config.ps1.tmpl", "aa:bb:cc:dd:ee:ff:ip:10.0.0.1,10.0.0.254,24,8.8.8.8,")
			Expect(err).NotTo(HaveOccurred())
			Expect(string(rendered)).To(ContainSubstring("aa:bb:cc:dd:ee:ff"))
		})

		It("returns error for missing template", func() {
			_, err := renderTemplate("nonexistent.ps1.tmpl", "data")
			Expect(err).To(HaveOccurred())
		})
	})
})
