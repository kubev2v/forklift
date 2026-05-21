package vmware

import (
	"testing"

	"github.com/kubev2v/forklift/pkg/virt-v2v/config"
	"github.com/kubev2v/forklift/pkg/virt-v2v/customization/api"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestVmwarePlugin(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Drivers/VMware Plugin Suite")
}

var _ = Describe("Plugin", func() {
	var p *Plugin

	BeforeEach(func() {
		p = &Plugin{}
	})

	Describe("Applicable", func() {
		It("returns true for Windows + vSphere + removal enabled", func() {
			ctx := &api.Context{
				Guest: &api.GuestInfo{OS: api.GuestOS{Family: api.OSFamilyWindows}},
				Config: &config.AppConfig{
					VsphereVmwareDriverRemoval: true,
					Source:                     config.VSPHERE,
				},
			}
			Expect(p.Applicable(ctx)).To(BeTrue())
		})

		It("returns false for Linux", func() {
			ctx := &api.Context{
				Guest: &api.GuestInfo{OS: api.GuestOS{Family: api.OSFamilyLinux, Distro: "rhel"}},
				Config: &config.AppConfig{
					VsphereVmwareDriverRemoval: true,
					Source:                     config.VSPHERE,
				},
			}
			Expect(p.Applicable(ctx)).To(BeFalse())
		})

		It("returns false for non-vSphere source", func() {
			ctx := &api.Context{
				Guest: &api.GuestInfo{OS: api.GuestOS{Family: api.OSFamilyWindows}},
				Config: &config.AppConfig{
					VsphereVmwareDriverRemoval: true,
					Source:                     config.OVA,
				},
			}
			Expect(p.Applicable(ctx)).To(BeFalse())
		})

		It("returns false when removal flag is disabled", func() {
			ctx := &api.Context{
				Guest: &api.GuestInfo{OS: api.GuestOS{Family: api.OSFamilyWindows}},
				Config: &config.AppConfig{
					VsphereVmwareDriverRemoval: false,
					Source:                     config.VSPHERE,
				},
			}
			Expect(p.Applicable(ctx)).To(BeFalse())
		})
	})

	Describe("Apply", func() {
		It("writes all driver removal scripts with ordered guest paths", func() {
			ctx := &api.Context{
				Guest:  &api.GuestInfo{OS: api.GuestOS{Family: api.OSFamilyWindows}},
				Config: &config.AppConfig{VsphereVmwareDriverRemoval: true, Source: config.VSPHERE},
			}

			actions, err := p.Apply(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(actions.Files).To(HaveLen(len(driverRemovalScripts)))
			for i, name := range driverRemovalScripts {
				Expect(actions.Files[i].Type).To(Equal(api.ActionWrite))
				Expect(actions.Files[i].GuestPath).To(ContainSubstring(name))
				Expect(actions.Files[i].Content).NotTo(BeEmpty())
			}
		})
	})
})
