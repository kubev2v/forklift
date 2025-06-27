package vsphere

import (
	modelVsphere "github.com/kubev2v/forklift/pkg/controller/provider/model/vsphere"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("SortNICsByGuestNetworkOrder", func() {
	var vm *modelVsphere.VM

	BeforeEach(func() {
		vm = &modelVsphere.VM{
			NICs:          []modelVsphere.NIC{},
			GuestNetworks: []modelVsphere.GuestNetwork{},
		}
	})

	Context("NICs match GuestNetworks order", func() {
		BeforeEach(func() {
			vm.NICs = []modelVsphere.NIC{
				{MAC: "00:11:22:33:44:55", Index: 0},
				{MAC: "66:77:88:99:AA:BB", Index: 1},
				{MAC: "CC:DD:EE:FF:00:11", Index: 2},
			}
			vm.GuestNetworks = []modelVsphere.GuestNetwork{
				{MAC: "66:77:88:99:AA:BB", Device: "0"},
				{MAC: "00:11:22:33:44:55", Device: "1"},
				{MAC: "CC:DD:EE:FF:00:11", Device: "2"},
			}
		})

		It("should reorder NICs to match GuestNetworks", func() {
			SortNICsByGuestNetworkOrder(vm)
			Expect(vm.NICs[0].MAC).To(Equal("66:77:88:99:AA:BB"))
			Expect(vm.NICs[1].MAC).To(Equal("00:11:22:33:44:55"))
			Expect(vm.NICs[2].MAC).To(Equal("CC:DD:EE:FF:00:11"))
		})
	})

	Context("Some NICs do not match GuestNetworks", func() {
		BeforeEach(func() {
			vm.NICs = []modelVsphere.NIC{
				{MAC: "00:11:22:33:44:55", Index: 0},
				{MAC: "66:77:88:99:AA:BB", Index: 1},
				{MAC: "CC:DD:EE:FF:00:11", Index: 2},
			}
			vm.GuestNetworks = []modelVsphere.GuestNetwork{
				{MAC: "66:77:88:99:AA:BB", Device: "0"},
				{MAC: "00:11:22:33:44:55", Device: "1"},
				// Missing "CC:DD:EE:FF:00:11"
			}
		})

		It("should reorder matching NICs and leave unmatched NICs at the end", func() {
			SortNICsByGuestNetworkOrder(vm)
			Expect(vm.NICs[0].MAC).To(Equal("66:77:88:99:AA:BB"))
			Expect(vm.NICs[1].MAC).To(Equal("00:11:22:33:44:55"))
			Expect(vm.NICs[2].MAC).To(Equal("CC:DD:EE:FF:00:11")) // Unmatched NIC remains
		})
	})

	Context("Empty NICs and GuestNetworks", func() {
		It("should not panic or modify anything", func() {
			Expect(func() {
				SortNICsByGuestNetworkOrder(vm)
			}).ToNot(Panic())
			Expect(vm.NICs).To(BeEmpty())
			Expect(vm.GuestNetworks).To(BeEmpty())
		})
	})
})
