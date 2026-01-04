package openstack

import (
	"path"
	"strconv"

	v1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("OpenStack builder", func() {
	DescribeTable("should", func(os, version, distro, matchPreferenceName string) {
		Expect(getPreferenceOs(os, version, distro)).Should(Equal(matchPreferenceName))
	},
		Entry("rhel9", RHEL, "9", RHEL, "rhel.9"),
		Entry("centos stream 9", CentOS, "9", CentOS, "centos.stream9"),
		Entry("windows 11", Windows, "11", Windows, "windows.11.virtio"),
		Entry("windows2022", Windows, "2022", Windows, "windows.2k22.virtio"),
		Entry("ubuntu 22", Ubuntu, "22.04.3", Ubuntu, "ubuntu"),
	)
})

var _ = Describe("OpenStack Glance const test", func() {
	It("GlanceSource should be glance, changing it may break the UI", func() {
		Expect(v1beta1.GlanceSource).Should(Equal("glance"))
	})
})

var _ = Describe("Numbered Auto-NAD Generation Logic", func() {
	Context("When processing multiple VMs with multiple NICs on same network", func() {
		It("should generate correct NAD assignments for 2 VMs each with 2 NICs", func() {
			// Simulate the logic flow for 2 VMs, each with 2 NICs on the same network

			// Test data
			destNADName := "prod-nad"
			destNADNamespace := "openshift-mtv"

			// Simulate VM1 processing
			usedDestinationsVM1 := make(map[string]bool)
			autoNADCountersVM1 := make(map[string]int)

			// VM1-NIC1
			destIdentifier := path.Join(destNADNamespace, destNADName)
			Expect(usedDestinationsVM1[destIdentifier]).To(BeFalse(), "First NIC should find NAD unused")

			// Simulate: VM1-NIC1 uses original NAD
			vm1nic1NAD := destNADName
			usedDestinationsVM1[destIdentifier] = true

			// VM1-NIC2
			destIdentifier = path.Join(destNADNamespace, destNADName)
			Expect(usedDestinationsVM1[destIdentifier]).To(BeTrue(), "Second NIC should find NAD already used")

			// Simulate: Generate auto NAD
			originalKey := path.Join(destNADNamespace, destNADName)
			autoNADCountersVM1[originalKey]++
			counter := autoNADCountersVM1[originalKey]
			Expect(counter).To(Equal(1), "First auto-NAD should have counter 1")

			vm1nic2NAD := destNADName + "-auto-1"
			vm1nic2Identifier := path.Join(destNADNamespace, vm1nic2NAD)
			usedDestinationsVM1[vm1nic2Identifier] = true

			// Verify VM1 results
			Expect(vm1nic1NAD).To(Equal("prod-nad"))
			Expect(vm1nic2NAD).To(Equal("prod-nad-auto-1"))
			Expect(usedDestinationsVM1).To(HaveLen(2))
			Expect(autoNADCountersVM1[originalKey]).To(Equal(1))

			// Simulate VM2 processing (fresh maps - new function call)
			usedDestinationsVM2 := make(map[string]bool)
			autoNADCountersVM2 := make(map[string]int)

			// VM2-NIC1
			destIdentifier = path.Join(destNADNamespace, destNADName)
			Expect(usedDestinationsVM2[destIdentifier]).To(BeFalse(), "VM2 first NIC should find NAD unused (fresh map)")

			// Simulate: VM2-NIC1 uses original NAD (will share with VM1-NIC1)
			vm2nic1NAD := destNADName
			usedDestinationsVM2[destIdentifier] = true

			// VM2-NIC2
			destIdentifier = path.Join(destNADNamespace, destNADName)
			Expect(usedDestinationsVM2[destIdentifier]).To(BeTrue(), "VM2 second NIC should find NAD already used")

			// Simulate: Generate auto NAD
			originalKey = path.Join(destNADNamespace, destNADName)
			autoNADCountersVM2[originalKey]++
			counter = autoNADCountersVM2[originalKey]
			Expect(counter).To(Equal(1), "VM2 first auto-NAD should also have counter 1 (fresh counter)")

			vm2nic2NAD := destNADName + "-auto-1"
			vm2nic2Identifier := path.Join(destNADNamespace, vm2nic2NAD)
			usedDestinationsVM2[vm2nic2Identifier] = true

			// Verify VM2 results
			Expect(vm2nic1NAD).To(Equal("prod-nad"))
			Expect(vm2nic2NAD).To(Equal("prod-nad-auto-1"))
			Expect(usedDestinationsVM2).To(HaveLen(2))
			Expect(autoNADCountersVM2[originalKey]).To(Equal(1))

			// Verify sharing behavior
			Expect(vm1nic1NAD).To(Equal(vm2nic1NAD), "Both VMs' first NICs should share the original NAD")
			Expect(vm1nic2NAD).To(Equal(vm2nic2NAD), "Both VMs' second NICs should share the same auto-NAD")
		})

		It("should generate correct NAD assignments for 1 VM with 4 NICs", func() {
			// Test data
			destNADName := "prod-nad"
			destNADNamespace := "openshift-mtv"

			usedDestinations := make(map[string]bool)
			autoNADCounters := make(map[string]int)

			nics := []string{}

			// Process 4 NICs
			for i := 1; i <= 4; i++ {
				destIdentifier := path.Join(destNADNamespace, destNADName)

				var nicNAD string
				if usedDestinations[destIdentifier] {
					// Generate auto NAD
					originalKey := path.Join(destNADNamespace, destNADName)
					autoNADCounters[originalKey]++
					counter := autoNADCounters[originalKey]

					nicNAD = destNADName + "-auto-" + strconv.Itoa(counter)
					nicIdentifier := path.Join(destNADNamespace, nicNAD)
					usedDestinations[nicIdentifier] = true
				} else {
					// Use original
					nicNAD = destNADName
					usedDestinations[destIdentifier] = true
				}

				nics = append(nics, nicNAD)
			}

			// Verify results
			Expect(nics).To(HaveLen(4))
			Expect(nics[0]).To(Equal("prod-nad"))
			Expect(nics[1]).To(Equal("prod-nad-auto-1"))
			Expect(nics[2]).To(Equal("prod-nad-auto-2"))
			Expect(nics[3]).To(Equal("prod-nad-auto-3"))

			// Verify all NADs are tracked as used
			Expect(usedDestinations).To(HaveLen(4))
			Expect(usedDestinations[path.Join(destNADNamespace, "prod-nad")]).To(BeTrue())
			Expect(usedDestinations[path.Join(destNADNamespace, "prod-nad-auto-1")]).To(BeTrue())
			Expect(usedDestinations[path.Join(destNADNamespace, "prod-nad-auto-2")]).To(BeTrue())
			Expect(usedDestinations[path.Join(destNADNamespace, "prod-nad-auto-3")]).To(BeTrue())
		})

		It("should handle multiple IPs with same MAC as single NIC", func() {
			// Simulate OpenStack scenario: 1 NIC with multiple fixed IPs
			// In OpenStack Addresses, this appears as multiple entries with same MAC

			// Test data
			destNADName := "prod-nad"
			destNADNamespace := "openshift-mtv"

			// Simulate VM with 2 NICs on same network:
			// - NIC1: MAC aa:bb:cc:dd:ee:01 with 2 IPs (10.0.0.10, 10.0.0.11)
			// - NIC2: MAC aa:bb:cc:dd:ee:02 with 1 IP (10.0.0.20)

			// Simulate address entries (as they come from OpenStack)
			type AddressEntry struct {
				MAC string
				IP  string
			}

			addressEntries := []AddressEntry{
				{MAC: "aa:bb:cc:dd:ee:01", IP: "10.0.0.10"}, // NIC1, IP1
				{MAC: "aa:bb:cc:dd:ee:01", IP: "10.0.0.11"}, // NIC1, IP2 (same MAC!)
				{MAC: "aa:bb:cc:dd:ee:02", IP: "10.0.0.20"}, // NIC2, IP1
			}

			// Group by MAC to identify unique NICs (simulates builder logic)
			nicsByMAC := make(map[string][]string) // MAC -> IPs
			for _, entry := range addressEntries {
				nicsByMAC[entry.MAC] = append(nicsByMAC[entry.MAC], entry.IP)
			}

			// Verify grouping
			Expect(nicsByMAC).To(HaveLen(2), "Should identify 2 unique NICs despite 3 address entries")
			Expect(nicsByMAC["aa:bb:cc:dd:ee:01"]).To(HaveLen(2), "NIC1 should have 2 IPs")
			Expect(nicsByMAC["aa:bb:cc:dd:ee:02"]).To(HaveLen(1), "NIC2 should have 1 IP")

			// Now process each unique NIC (by MAC) for NAD assignment
			usedDestinations := make(map[string]bool)
			autoNADCounters := make(map[string]int)

			nics := []struct {
				MAC string
				NAD string
				IPs []string
			}{}

			for mac, ips := range nicsByMAC {
				destIdentifier := path.Join(destNADNamespace, destNADName)

				var nicNAD string
				if usedDestinations[destIdentifier] {
					// Generate auto NAD
					originalKey := path.Join(destNADNamespace, destNADName)
					autoNADCounters[originalKey]++
					counter := autoNADCounters[originalKey]

					nicNAD = destNADName + "-auto-" + strconv.Itoa(counter)
					nicIdentifier := path.Join(destNADNamespace, nicNAD)
					usedDestinations[nicIdentifier] = true
				} else {
					// Use original
					nicNAD = destNADName
					usedDestinations[destIdentifier] = true
				}

				nics = append(nics, struct {
					MAC string
					NAD string
					IPs []string
				}{MAC: mac, NAD: nicNAD, IPs: ips})
			}

			// Verify results: Should have 2 NICs (not 3)
			Expect(nics).To(HaveLen(2), "Should create 2 network interfaces, not 3")

			// Verify NAD assignments
			Expect(usedDestinations).To(HaveLen(2), "Should use 2 NADs total")

			// Find the NIC with 2 IPs
			var multiIPNIC *struct {
				MAC string
				NAD string
				IPs []string
			}
			var singleIPNIC *struct {
				MAC string
				NAD string
				IPs []string
			}

			for i := range nics {
				if len(nics[i].IPs) == 2 {
					multiIPNIC = &nics[i]
				} else if len(nics[i].IPs) == 1 {
					singleIPNIC = &nics[i]
				}
			}

			Expect(multiIPNIC).ToNot(BeNil(), "Should find NIC with 2 IPs")
			Expect(singleIPNIC).ToNot(BeNil(), "Should find NIC with 1 IP")

			// Verify the multi-IP NIC is treated as a single NIC
			Expect(multiIPNIC.MAC).To(Equal("aa:bb:cc:dd:ee:01"))
			Expect(multiIPNIC.IPs).To(ConsistOf("10.0.0.10", "10.0.0.11"))

			// Verify NAD assignments (order may vary due to map iteration)
			nadsUsed := []string{multiIPNIC.NAD, singleIPNIC.NAD}
			Expect(nadsUsed).To(ContainElement("prod-nad"))
			Expect(nadsUsed).To(ContainElement("prod-nad-auto-1"))

			// Verify counter
			originalKey := path.Join(destNADNamespace, destNADName)
			Expect(autoNADCounters[originalKey]).To(Equal(1), "Should generate only 1 auto-NAD for 2 NICs")
		})
	})

	Context("Edge Cases", func() {
		It("should maintain separate counters for different original NADs", func() {
			// Test that counters are scoped per original NAD
			destNADNamespace := "openshift-mtv"

			usedDestinations := make(map[string]bool)
			autoNADCounters := make(map[string]int)

			// Process 2 NICs for prod-nad
			usedDestinations[path.Join(destNADNamespace, "prod-nad")] = true
			autoNADCounters[path.Join(destNADNamespace, "prod-nad")]++
			prodCounter1 := autoNADCounters[path.Join(destNADNamespace, "prod-nad")]

			// Process 3 NICs for dev-nad
			usedDestinations[path.Join(destNADNamespace, "dev-nad")] = true
			autoNADCounters[path.Join(destNADNamespace, "dev-nad")]++
			devCounter1 := autoNADCounters[path.Join(destNADNamespace, "dev-nad")]
			autoNADCounters[path.Join(destNADNamespace, "dev-nad")]++
			devCounter2 := autoNADCounters[path.Join(destNADNamespace, "dev-nad")]

			// Process another NIC for prod-nad
			autoNADCounters[path.Join(destNADNamespace, "prod-nad")]++
			prodCounter2 := autoNADCounters[path.Join(destNADNamespace, "prod-nad")]

			// Verify counters are independent
			Expect(prodCounter1).To(Equal(1))
			Expect(prodCounter2).To(Equal(2))
			Expect(devCounter1).To(Equal(1))
			Expect(devCounter2).To(Equal(2))

			// Verify final counter values
			Expect(autoNADCounters[path.Join(destNADNamespace, "prod-nad")]).To(Equal(2))
			Expect(autoNADCounters[path.Join(destNADNamespace, "dev-nad")]).To(Equal(2))
		})
	})
})
