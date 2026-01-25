package mapping

import (
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

func TestMapping(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "EC2 controller mapping")
}

var _ = Describe("EC2 Controller Mapping", func() {
	Describe("FindStorageClass", func() {
		It("should find storage class for volume type", func() {
			storageMap := &api.StorageMap{
				Spec: api.StorageMapSpec{
					Map: []api.StoragePair{
						{
							Source:      ref.Ref{Name: "gp2"},
							Destination: api.DestinationStorage{StorageClass: "standard"},
						},
						{
							Source:      ref.Ref{Name: "gp3"},
							Destination: api.DestinationStorage{StorageClass: "premium-rwo"},
						},
					},
				},
			}

			result := FindStorageClass(storageMap, "gp3")

			Expect(result).To(Equal("premium-rwo"))
		})

		It("should return empty string when volume type not found", func() {
			storageMap := &api.StorageMap{
				Spec: api.StorageMapSpec{
					Map: []api.StoragePair{
						{
							Source:      ref.Ref{Name: "gp2"},
							Destination: api.DestinationStorage{StorageClass: "standard"},
						},
					},
				},
			}

			result := FindStorageClass(storageMap, "io1")

			Expect(result).To(Equal(""))
		})

		It("should return empty string when storageMap is nil", func() {
			result := FindStorageClass(nil, "gp2")

			Expect(result).To(Equal(""))
		})

		It("should return empty string when map is empty", func() {
			storageMap := &api.StorageMap{
				Spec: api.StorageMapSpec{
					Map: []api.StoragePair{},
				},
			}

			result := FindStorageClass(storageMap, "gp2")

			Expect(result).To(Equal(""))
		})
	})

	Describe("HasStorageMapping", func() {
		var storageMap *api.StorageMap

		BeforeEach(func() {
			storageMap = &api.StorageMap{
				Spec: api.StorageMapSpec{
					Map: []api.StoragePair{
						{
							Source:      ref.Ref{Name: "gp2"},
							Destination: api.DestinationStorage{StorageClass: "standard"},
						},
						{
							Source:      ref.Ref{Name: "gp3"},
							Destination: api.DestinationStorage{StorageClass: "premium-rwo"},
						},
						{
							Source:      ref.Ref{Name: "io1"},
							Destination: api.DestinationStorage{StorageClass: "io-optimized"},
						},
					},
				},
			}
		})

		table.DescribeTable("should check mapping existence",
			func(volumeType string, expected bool) {
				Expect(HasStorageMapping(storageMap, volumeType)).To(Equal(expected))
			},
			table.Entry("gp2 exists", "gp2", true),
			table.Entry("gp3 exists", "gp3", true),
			table.Entry("io1 exists", "io1", true),
			table.Entry("io2 does not exist", "io2", false),
			table.Entry("st1 does not exist", "st1", false),
			table.Entry("empty string does not exist", "", false),
		)

		It("should return false for nil storageMap", func() {
			Expect(HasStorageMapping(nil, "gp2")).To(BeFalse())
		})
	})

	Describe("FindNetworkPair", func() {
		It("should find network pair by Source.ID", func() {
			networkMap := &api.NetworkMap{
				Spec: api.NetworkMapSpec{
					Map: []api.NetworkPair{
						{
							Source: ref.Ref{ID: "subnet-123", Name: ""},
							Destination: api.DestinationNetwork{
								Type: "pod",
							},
						},
					},
				},
			}

			result := FindNetworkPair(networkMap, "subnet-123")

			Expect(result).NotTo(BeNil())
			Expect(result.Source.ID).To(Equal("subnet-123"))
		})

		It("should find network pair by Source.Name", func() {
			networkMap := &api.NetworkMap{
				Spec: api.NetworkMapSpec{
					Map: []api.NetworkPair{
						{
							Source: ref.Ref{ID: "", Name: "subnet-456"},
							Destination: api.DestinationNetwork{
								Type: "pod",
							},
						},
					},
				},
			}

			result := FindNetworkPair(networkMap, "subnet-456")

			Expect(result).NotTo(BeNil())
			Expect(result.Source.Name).To(Equal("subnet-456"))
		})

		It("should return nil when subnet not found", func() {
			networkMap := &api.NetworkMap{
				Spec: api.NetworkMapSpec{
					Map: []api.NetworkPair{
						{
							Source: ref.Ref{ID: "subnet-123"},
							Destination: api.DestinationNetwork{
								Type: "pod",
							},
						},
					},
				},
			}

			result := FindNetworkPair(networkMap, "subnet-nonexistent")

			Expect(result).To(BeNil())
		})

		It("should return nil when networkMap is nil", func() {
			result := FindNetworkPair(nil, "subnet-123")

			Expect(result).To(BeNil())
		})

		It("should return nil when networkMap.Spec.Map is nil", func() {
			networkMap := &api.NetworkMap{
				Spec: api.NetworkMapSpec{
					Map: nil,
				},
			}

			result := FindNetworkPair(networkMap, "subnet-123")

			Expect(result).To(BeNil())
		})

		It("should return nil when map is empty", func() {
			networkMap := &api.NetworkMap{
				Spec: api.NetworkMapSpec{
					Map: []api.NetworkPair{},
				},
			}

			result := FindNetworkPair(networkMap, "subnet-123")

			Expect(result).To(BeNil())
		})
	})

	Describe("HasNetworkMapping", func() {
		var networkMap *api.NetworkMap

		BeforeEach(func() {
			networkMap = &api.NetworkMap{
				Spec: api.NetworkMapSpec{
					Map: []api.NetworkPair{
						{
							// Both ID and Name set to prevent empty string matching
							Source: ref.Ref{ID: "subnet-111", Name: "subnet-111-name"},
							Destination: api.DestinationNetwork{
								Type: "pod",
							},
						},
						{
							Source: ref.Ref{ID: "subnet-222-id", Name: "subnet-222"},
							Destination: api.DestinationNetwork{
								Type: "multus",
							},
						},
					},
				},
			}
		})

		table.DescribeTable("should check mapping existence",
			func(subnetID string, expected bool) {
				Expect(HasNetworkMapping(networkMap, subnetID)).To(Equal(expected))
			},
			table.Entry("subnet-111 exists by ID", "subnet-111", true),
			table.Entry("subnet-222 exists by Name", "subnet-222", true),
			table.Entry("subnet-333 does not exist", "subnet-333", false),
			table.Entry("empty string does not exist", "", false),
		)

		It("should return false for nil networkMap", func() {
			Expect(HasNetworkMapping(nil, "subnet-111")).To(BeFalse())
		})
	})

	Describe("Multiple mappings", func() {
		It("should find first matching network pair", func() {
			networkMap := &api.NetworkMap{
				Spec: api.NetworkMapSpec{
					Map: []api.NetworkPair{
						{
							Source: ref.Ref{ID: "subnet-123"},
							Destination: api.DestinationNetwork{
								Type: "pod",
							},
						},
						{
							Source: ref.Ref{ID: "subnet-123"}, // Duplicate
							Destination: api.DestinationNetwork{
								Type: "multus",
							},
						},
					},
				},
			}

			result := FindNetworkPair(networkMap, "subnet-123")

			Expect(result).NotTo(BeNil())
			Expect(result.Destination.Type).To(Equal("pod")) // First match
		})

		It("should handle all common EBS volume types", func() {
			storageMap := &api.StorageMap{
				Spec: api.StorageMapSpec{
					Map: []api.StoragePair{
						{Source: ref.Ref{Name: "gp2"}, Destination: api.DestinationStorage{StorageClass: "gp2-sc"}},
						{Source: ref.Ref{Name: "gp3"}, Destination: api.DestinationStorage{StorageClass: "gp3-sc"}},
						{Source: ref.Ref{Name: "io1"}, Destination: api.DestinationStorage{StorageClass: "io1-sc"}},
						{Source: ref.Ref{Name: "io2"}, Destination: api.DestinationStorage{StorageClass: "io2-sc"}},
						{Source: ref.Ref{Name: "st1"}, Destination: api.DestinationStorage{StorageClass: "st1-sc"}},
						{Source: ref.Ref{Name: "sc1"}, Destination: api.DestinationStorage{StorageClass: "sc1-sc"}},
						{Source: ref.Ref{Name: "standard"}, Destination: api.DestinationStorage{StorageClass: "standard-sc"}},
					},
				},
			}

			volumeTypes := []string{"gp2", "gp3", "io1", "io2", "st1", "sc1", "standard"}
			for _, vt := range volumeTypes {
				Expect(HasStorageMapping(storageMap, vt)).To(BeTrue(), "Expected mapping for %s", vt)
				Expect(FindStorageClass(storageMap, vt)).To(Equal(vt + "-sc"))
			}
		})
	})
})
