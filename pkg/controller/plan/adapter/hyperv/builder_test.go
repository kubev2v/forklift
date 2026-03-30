package hyperv

import (
	v1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/controller/provider/model/hyperv"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/hyperv"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var builderLog = logging.WithName("hyperv-builder-test")

var _ = Describe("HyperV builder", func() {
	Context("mapMacStaticIps", func() {
		buildBuilder := func() *Builder {
			return &Builder{
				Context: &plancontext.Context{
					Plan: createTestPlan(),
					Log:  builderLog,
				},
			}
		}

		DescribeTable("should map static IPs correctly",
			func(vm *model.VM, excludeMACs map[string]bool, expected string) {
				b := buildBuilder()
				result := b.mapMacStaticIps(vm, excludeMACs)
				Expect(result).To(Equal(expected))
			},

			Entry("no guest networks", &model.VM{GuestOS: "windows"}, nil, ""),
			Entry("Linux VM with static IP", &model.VM{
				GuestOS: "Ubuntu",
				GuestNetworks: []hyperv.GuestNetwork{
					{MAC: "00:15:5D:01:02:03", IP: "192.168.1.10", Origin: hyperv.OriginManual, PrefixLength: 24, Gateway: "192.168.1.1", DNS: []string{"8.8.8.8"}},
				},
			}, nil, "00:15:5D:01:02:03:ip:192.168.1.10,192.168.1.1,24,8.8.8.8"),
			Entry("Windows VM with manual origin", &model.VM{
				GuestOS: "Windows Server 2022",
				GuestNetworks: []hyperv.GuestNetwork{
					{MAC: "00:15:5D:01:02:03", IP: "10.0.0.5", Origin: hyperv.OriginManual, PrefixLength: 24, Gateway: "10.0.0.1", DNS: []string{"10.0.0.2"}},
				},
			}, nil, "00:15:5D:01:02:03:ip:10.0.0.5,10.0.0.1,24,10.0.0.2"),
			Entry("Windows VM skips DHCP", &model.VM{
				GuestOS: "Windows Server 2022",
				GuestNetworks: []hyperv.GuestNetwork{
					{MAC: "00:15:5D:01:02:03", IP: "10.0.0.5", Origin: hyperv.OriginDhcp, PrefixLength: 24, Gateway: "10.0.0.1"},
				},
			}, nil, ""),
			Entry("exclude single NIC by MAC", &model.VM{
				GuestOS: "Ubuntu",
				GuestNetworks: []hyperv.GuestNetwork{
					{MAC: "00:15:5D:01:02:03", IP: "192.168.1.10", Origin: hyperv.OriginManual, PrefixLength: 24, Gateway: "192.168.1.1"},
					{MAC: "00:15:5D:01:02:04", IP: "192.168.1.11", Origin: hyperv.OriginManual, PrefixLength: 24, Gateway: "192.168.1.1"},
				},
			}, map[string]bool{"00:15:5d:01:02:03": true}, "00:15:5D:01:02:04:ip:192.168.1.11,192.168.1.1,24"),
			Entry("exclude all NICs", &model.VM{
				GuestOS: "Ubuntu",
				GuestNetworks: []hyperv.GuestNetwork{
					{MAC: "00:15:5D:01:02:03", IP: "192.168.1.10", Origin: hyperv.OriginManual, PrefixLength: 24, Gateway: "192.168.1.1"},
					{MAC: "00:15:5D:01:02:04", IP: "192.168.1.11", Origin: hyperv.OriginManual, PrefixLength: 24, Gateway: "192.168.1.1"},
				},
			}, map[string]bool{"00:15:5d:01:02:03": true, "00:15:5d:01:02:04": true}, ""),
			Entry("exclude with hyphen-separated MAC (normalized)", &model.VM{
				GuestOS: "Ubuntu",
				GuestNetworks: []hyperv.GuestNetwork{
					{MAC: "00-15-5D-01-02-03", IP: "192.168.1.10", Origin: hyperv.OriginManual, PrefixLength: 24, Gateway: "192.168.1.1"},
					{MAC: "00:15:5D:01:02:04", IP: "192.168.1.11", Origin: hyperv.OriginManual, PrefixLength: 24, Gateway: "192.168.1.1"},
				},
			}, map[string]bool{"00:15:5d:01:02:03": true}, "00:15:5D:01:02:04:ip:192.168.1.11,192.168.1.1,24"),
			Entry("empty exclusion list is same as nil", &model.VM{
				GuestOS: "Ubuntu",
				GuestNetworks: []hyperv.GuestNetwork{
					{MAC: "00:15:5D:01:02:03", IP: "192.168.1.10", Origin: hyperv.OriginManual, PrefixLength: 24, Gateway: "192.168.1.1"},
				},
			}, map[string]bool{}, "00:15:5D:01:02:03:ip:192.168.1.10,192.168.1.1,24"),
		)
	})

	Context("PodEnvironment per-VM preserveStaticIPs", func() {
		buildTestBuilder := func(planPreserve bool, vmOverride *bool, vm model.VM) *Builder {
			planVMs := []planapi.VM{
				{
					Ref:               ref.Ref{ID: vm.ID, Name: vm.Name},
					PreserveStaticIPs: vmOverride,
				},
			}
			plan := createTestPlan()
			plan.Spec.PreserveStaticIPs = planPreserve
			plan.Spec.VMs = planVMs

			return &Builder{
				Context: &plancontext.Context{
					Plan: plan,
					Source: plancontext.Source{
						Inventory: &mockHypervInventory{vm: vm},
					},
					Log: builderLog,
				},
			}
		}

		It("should not produce V2V_staticIPs when VM overrides to false", func() {
			vm := model.VM{
				VM1: model.VM1{
					VM0: model.Resource{ID: "vm-1", Name: "test-vm"},
					Disks: []hyperv.Disk{
						{Base: hyperv.Base{ID: "disk-1"}, SMBPath: "/hyperv/disk1.vhdx", Capacity: 1024},
					},
				},
				GuestOS: "Ubuntu",
				GuestNetworks: []hyperv.GuestNetwork{
					{MAC: "00:15:5D:01:02:03", IP: "192.168.1.10", Origin: hyperv.OriginManual, PrefixLength: 24, Gateway: "192.168.1.1"},
				},
			}
			b := buildTestBuilder(true, ptr.To(false), vm)
			env, err := b.PodEnvironment(ref.Ref{ID: "vm-1", Name: "test-vm"}, &core.Secret{})
			Expect(err).NotTo(HaveOccurred())
			for _, e := range env {
				Expect(e.Name).NotTo(Equal("V2V_staticIPs"))
			}
		})

		It("should produce V2V_staticIPs when VM overrides to true and plan is false", func() {
			vm := model.VM{
				VM1: model.VM1{
					VM0: model.Resource{ID: "vm-1", Name: "test-vm"},
					Disks: []hyperv.Disk{
						{Base: hyperv.Base{ID: "disk-1"}, SMBPath: "/hyperv/disk1.vhdx", Capacity: 1024},
					},
				},
				GuestOS: "Ubuntu",
				GuestNetworks: []hyperv.GuestNetwork{
					{MAC: "00:15:5D:01:02:03", IP: "192.168.1.10", Origin: hyperv.OriginManual, PrefixLength: 24, Gateway: "192.168.1.1"},
				},
			}
			b := buildTestBuilder(false, ptr.To(true), vm)
			env, err := b.PodEnvironment(ref.Ref{ID: "vm-1", Name: "test-vm"}, &core.Secret{})
			Expect(err).NotTo(HaveOccurred())
			var foundStaticIPs bool
			for _, e := range env {
				if e.Name == "V2V_staticIPs" {
					foundStaticIPs = true
					Expect(e.Value).To(ContainSubstring("00:15:5D:01:02:03"))
				}
			}
			Expect(foundStaticIPs).To(BeTrue())
		})

		It("should inherit plan-level preserveStaticIPs when VM override is nil", func() {
			vm := model.VM{
				VM1: model.VM1{
					VM0: model.Resource{ID: "vm-1", Name: "test-vm"},
					Disks: []hyperv.Disk{
						{Base: hyperv.Base{ID: "disk-1"}, SMBPath: "/hyperv/disk1.vhdx", Capacity: 1024},
					},
				},
				GuestOS: "Ubuntu",
				GuestNetworks: []hyperv.GuestNetwork{
					{MAC: "00:15:5D:01:02:03", IP: "192.168.1.10", Origin: hyperv.OriginManual, PrefixLength: 24, Gateway: "192.168.1.1"},
				},
			}
			b := buildTestBuilder(true, nil, vm)
			env, err := b.PodEnvironment(ref.Ref{ID: "vm-1", Name: "test-vm"}, &core.Secret{})
			Expect(err).NotTo(HaveOccurred())
			var foundStaticIPs bool
			for _, e := range env {
				if e.Name == "V2V_staticIPs" {
					foundStaticIPs = true
				}
			}
			Expect(foundStaticIPs).To(BeTrue())
		})
	})
})

func createTestPlan() *v1beta1.Plan {
	return &v1beta1.Plan{
		ObjectMeta: meta.ObjectMeta{
			Name:      "test-hyperv-plan",
			Namespace: "test",
		},
		Spec: v1beta1.PlanSpec{
			TargetNamespace:   "test",
			PreserveStaticIPs: true,
			VMs:               []planapi.VM{{Ref: ref.Ref{Name: "test-vm", ID: "vm-1"}}},
		},
		Referenced: v1beta1.Referenced{
			Provider: struct {
				Source      *v1beta1.Provider
				Destination *v1beta1.Provider
			}{
				Source: &v1beta1.Provider{
					ObjectMeta: meta.ObjectMeta{Name: "test-hyperv-provider"},
				},
				Destination: &v1beta1.Provider{},
			},
		},
	}
}
