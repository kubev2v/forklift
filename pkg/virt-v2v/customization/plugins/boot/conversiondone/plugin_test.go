package conversiondone

import (
	"testing"

	"github.com/kubev2v/forklift/pkg/virt-v2v/config"
	"github.com/kubev2v/forklift/pkg/virt-v2v/customization/api"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestConversionDonePlugin(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Boot/ConversionDone Plugin Suite")
}

var _ = Describe("Plugin", func() {
	var p *Plugin

	BeforeEach(func() {
		p = &Plugin{}
	})

	Describe("Applicable", func() {
		It("returns true for Windows with WaitForGuestReboot", func() {
			ctx := &api.Context{
				Guest:  &api.GuestInfo{OS: api.GuestOS{Family: api.OSFamilyWindows}},
				Config: &config.AppConfig{WaitForGuestReboot: true},
			}
			Expect(p.Applicable(ctx)).To(BeTrue())
		})

		It("returns false when WaitForGuestReboot is false", func() {
			ctx := &api.Context{
				Guest:  &api.GuestInfo{OS: api.GuestOS{Family: api.OSFamilyWindows}},
				Config: &config.AppConfig{},
			}
			Expect(p.Applicable(ctx)).To(BeFalse())
		})

		It("returns false for Linux even with WaitForGuestReboot", func() {
			ctx := &api.Context{
				Guest:  &api.GuestInfo{OS: api.GuestOS{Family: api.OSFamilyLinux, Distro: "rhel"}},
				Config: &config.AppConfig{WaitForGuestReboot: true},
			}
			Expect(p.Applicable(ctx)).To(BeFalse())
		})
	})

	Describe("Apply", func() {
		It("writes the signal conversion done script from embedded FS", func() {
			ctx := &api.Context{
				Guest:  &api.GuestInfo{OS: api.GuestOS{Family: api.OSFamilyWindows}},
				Config: &config.AppConfig{WaitForGuestReboot: true},
			}

			actions, err := p.Apply(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(actions.Files).To(HaveLen(1))
			Expect(actions.Files[0].Type).To(Equal(api.ActionWrite))
			Expect(actions.Files[0].GuestPath).To(Equal("/Program Files/Guestfs/Firstboot/scripts/990_signal_conversion_done.ps1"))
			Expect(actions.Files[0].Content).NotTo(BeEmpty())
		})
	})
})
