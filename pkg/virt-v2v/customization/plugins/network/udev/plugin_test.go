package udev

import (
	"testing"

	"github.com/kubev2v/forklift/pkg/virt-v2v/config"
	"github.com/kubev2v/forklift/pkg/virt-v2v/customization/api"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestUdevPlugin(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Network/Udev Plugin Suite")
}

var _ = Describe("Plugin", func() {
	var p *Plugin

	BeforeEach(func() {
		p = &Plugin{}
	})

	Describe("Applicable", func() {
		It("returns true for Linux with StaticIPs and Interfaces", func() {
			ctx := &api.Context{
				Guest: &api.GuestInfo{
					OS:         api.GuestOS{Family: api.OSFamilyLinux, Distro: "rhel"},
					Interfaces: []api.InterfaceInfo{{Name: "eth0", IPv4: []string{"192.168.1.10"}}},
				},
				Config: &config.AppConfig{StaticIPs: "00:11:22:33:44:55:ip:192.168.1.10"},
			}
			Expect(p.Applicable(ctx)).To(BeTrue())
		})

		It("returns false for Windows", func() {
			ctx := &api.Context{
				Guest: &api.GuestInfo{
					OS:         api.GuestOS{Family: api.OSFamilyWindows},
					Interfaces: []api.InterfaceInfo{{Name: "eth0", IPv4: []string{"192.168.1.10"}}},
				},
				Config: &config.AppConfig{StaticIPs: "00:11:22:33:44:55:ip:192.168.1.10"},
			}
			Expect(p.Applicable(ctx)).To(BeFalse())
		})

		It("returns false when no static IPs configured", func() {
			ctx := &api.Context{
				Guest: &api.GuestInfo{
					OS:         api.GuestOS{Family: api.OSFamilyLinux},
					Interfaces: []api.InterfaceInfo{{Name: "eth0", IPv4: []string{"192.168.1.10"}}},
				},
				Config: &config.AppConfig{},
			}
			Expect(p.Applicable(ctx)).To(BeFalse())
		})

		It("returns false when no interfaces extracted", func() {
			ctx := &api.Context{
				Guest: &api.GuestInfo{
					OS:         api.GuestOS{Family: api.OSFamilyLinux},
					Interfaces: nil,
				},
				Config: &config.AppConfig{StaticIPs: "00:11:22:33:44:55:ip:192.168.1.10"},
			}
			Expect(p.Applicable(ctx)).To(BeFalse())
		})
	})

	Describe("Apply", func() {
		It("generates udev rules for matching IP/MAC pairs", func() {
			ctx := &api.Context{
				Guest: &api.GuestInfo{
					OS: api.GuestOS{Family: api.OSFamilyLinux},
					Interfaces: []api.InterfaceInfo{
						{Name: "eth0", IPv4: []string{"192.168.1.10"}},
						{Name: "eth1", IPv4: []string{"10.0.0.5"}},
					},
				},
				Config: &config.AppConfig{
					StaticIPs: "00:11:22:33:44:55:ip:192.168.1.10,192.168.1.1,24,8.8.8.8_aa:bb:cc:dd:ee:ff:ip:10.0.0.5,10.0.0.1,24,1.1.1.1",
				},
			}

			actions, err := p.Apply(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(actions.Files).To(HaveLen(1))
			rule := string(actions.Files[0].Content)
			Expect(rule).To(ContainSubstring(`ATTR{address}=="00:11:22:33:44:55",NAME="eth0"`))
			Expect(rule).To(ContainSubstring(`ATTR{address}=="aa:bb:cc:dd:ee:ff",NAME="eth1"`))
			Expect(actions.Files[0].GuestPath).To(Equal("/etc/udev/rules.d/70-persistent-net.rules"))
			Expect(actions.Files[0].Permissions).To(Equal("0644"))
		})

		It("returns empty actions when no IPs match", func() {
			ctx := &api.Context{
				Guest: &api.GuestInfo{
					OS: api.GuestOS{Family: api.OSFamilyLinux},
					Interfaces: []api.InterfaceInfo{
						{Name: "eth0", IPv4: []string{"192.168.1.10"}},
					},
				},
				Config: &config.AppConfig{
					StaticIPs: "00:11:22:33:44:55:ip:10.99.99.99,10.0.0.1,24,8.8.8.8",
				},
			}

			actions, err := p.Apply(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(actions.Files).To(BeEmpty())
		})
	})

	Describe("parseStaticIPs", func() {
		It("parses single MAC:IP pair", func() {
			pairs := parseStaticIPs("00:11:22:33:44:55:ip:192.168.1.10,192.168.1.1,24,8.8.8.8")
			Expect(pairs).To(HaveLen(1))
			Expect(pairs[0].MAC).To(Equal("00:11:22:33:44:55"))
			Expect(pairs[0].IP).To(Equal("192.168.1.10"))
		})

		It("parses multiple MAC:IP pairs separated by underscore", func() {
			pairs := parseStaticIPs("00:11:22:33:44:55:ip:192.168.1.10,gw,24,dns_aa:bb:cc:dd:ee:ff:ip:10.0.0.5,gw,16,dns")
			Expect(pairs).To(HaveLen(2))
			Expect(pairs[0].MAC).To(Equal("00:11:22:33:44:55"))
			Expect(pairs[0].IP).To(Equal("192.168.1.10"))
			Expect(pairs[1].MAC).To(Equal("aa:bb:cc:dd:ee:ff"))
			Expect(pairs[1].IP).To(Equal("10.0.0.5"))
		})

		It("returns empty for invalid format", func() {
			pairs := parseStaticIPs("invalid-format")
			Expect(pairs).To(BeEmpty())
		})
	})
})
