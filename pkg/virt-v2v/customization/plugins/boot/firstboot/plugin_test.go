package firstboot

import (
	"testing"

	"github.com/kubev2v/forklift/pkg/virt-v2v/config"
	"github.com/kubev2v/forklift/pkg/virt-v2v/customization/api"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestFirstbootPlugin(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Boot/Firstboot Plugin Suite")
}

var _ = Describe("Plugin", func() {
	var p *Plugin

	BeforeEach(func() {
		p = &Plugin{}
	})

	Describe("Applicable", func() {
		It("returns true for Windows", func() {
			ctx := &api.Context{
				Guest:  &api.GuestInfo{OS: api.GuestOS{Family: api.OSFamilyWindows}},
				Config: &config.AppConfig{},
			}
			Expect(p.Applicable(ctx)).To(BeTrue())
		})

		It("returns false for Linux", func() {
			ctx := &api.Context{
				Guest:  &api.GuestInfo{OS: api.GuestOS{Family: api.OSFamilyLinux, Distro: "rhel"}},
				Config: &config.AppConfig{},
			}
			Expect(p.Applicable(ctx)).To(BeFalse())
		})
	})

	Describe("Apply", func() {
		It("writes standard firstboot bat for non-legacy", func() {
			ctx := &api.Context{
				Guest:  &api.GuestInfo{OS: api.GuestOS{Family: api.OSFamilyWindows}},
				Config: &config.AppConfig{},
			}

			actions, err := p.Apply(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(actions.Files).To(HaveLen(1))
			Expect(actions.Files[0].Type).To(Equal(api.ActionWrite))
			Expect(actions.Files[0].GuestPath).To(Equal("/Program Files/Guestfs/Firstboot/scripts/900_firstboot_init.bat"))
			Expect(actions.Files[0].Content).NotTo(BeEmpty())
		})

		It("writes legacy firstboot bat when legacy drivers set", func() {
			ctx := &api.Context{
				Guest:  &api.GuestInfo{OS: api.GuestOS{Family: api.OSFamilyWindows}},
				Config: &config.AppConfig{VirtIoWinLegacyDrivers: "/mnt/virtio.iso"},
			}

			actions, err := p.Apply(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(actions.Files).To(HaveLen(1))
			Expect(actions.Files[0].Type).To(Equal(api.ActionWrite))
			Expect(actions.Files[0].GuestPath).To(Equal("/Program Files/Guestfs/Firstboot/scripts/900_firstboot_init_legacy.bat"))
			Expect(actions.Files[0].Content).NotTo(BeEmpty())
		})
	})
})
