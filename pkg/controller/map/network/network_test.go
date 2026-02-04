// Generated-by: Claude
package network

import (
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func TestNetwork(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Network Map Suite")
}

var _ = Describe("NetworkMap Struct", func() {
	Describe("Status Conditions", func() {
		It("should set and check conditions", func() {
			nm := &api.NetworkMap{
				ObjectMeta: meta.ObjectMeta{
					Name:      "test-network-map",
					Namespace: "default",
				},
			}
			nm.Status.SetCondition(libcnd.Condition{
				Type:     SourceNetworkNotValid,
				Status:   True,
				Reason:   NotFound,
				Category: Critical,
				Message:  "Source network not found.",
			})
			Expect(nm.Status.HasCondition(SourceNetworkNotValid)).To(BeTrue())
			Expect(nm.Status.HasCondition(DestinationNetworkNotValid)).To(BeFalse())
		})

		It("should check multiple conditions with HasAnyCondition", func() {
			nm := &api.NetworkMap{
				ObjectMeta: meta.ObjectMeta{
					Name:      "test-network-map",
					Namespace: "default",
				},
			}
			nm.Status.SetCondition(libcnd.Condition{
				Type:     SourceNetworkNotValid,
				Status:   True,
				Reason:   NotFound,
				Category: Critical,
				Message:  "Source network not found.",
			})
			nm.Status.SetCondition(libcnd.Condition{
				Type:     DestinationNetworkNotValid,
				Status:   True,
				Reason:   NotFound,
				Category: Critical,
				Message:  "Destination network not found.",
			})
			Expect(nm.Status.HasAnyCondition(SourceNetworkNotValid, DestinationNetworkNotValid)).To(BeTrue())
		})
	})
})

var _ = Describe("NetworkPair", func() {
	Describe("Source Ref NotSet", func() {
		It("should return true when source ref is empty", func() {
			pair := api.NetworkPair{
				Source: ref.Ref{},
				Destination: api.DestinationNetwork{
					Type: Pod,
				},
			}
			Expect(pair.Source.NotSet()).To(BeTrue())
		})

		It("should return false when source has ID", func() {
			pair := api.NetworkPair{
				Source: ref.Ref{ID: "network-id"},
				Destination: api.DestinationNetwork{
					Type: Pod,
				},
			}
			Expect(pair.Source.NotSet()).To(BeFalse())
		})

		It("should return false when source has Name", func() {
			pair := api.NetworkPair{
				Source: ref.Ref{Name: "network-name"},
				Destination: api.DestinationNetwork{
					Type: Pod,
				},
			}
			Expect(pair.Source.NotSet()).To(BeFalse())
		})
	})
})

var _ = Describe("NetworkMap Methods", func() {
	var nm *api.NetworkMap

	BeforeEach(func() {
		nm = &api.NetworkMap{
			ObjectMeta: meta.ObjectMeta{
				Name:      "test-network-map",
				Namespace: "default",
			},
			Spec: api.NetworkMapSpec{
				Map: []api.NetworkPair{
					{
						Source: ref.Ref{
							ID:   "network-1",
							Name: "network-one",
							Type: "bridge",
						},
						Destination: api.DestinationNetwork{
							Type: Pod,
						},
					},
					{
						Source: ref.Ref{
							ID:        "network-2",
							Name:      "network-two",
							Namespace: "source-ns",
						},
						Destination: api.DestinationNetwork{
							Type:      Multus,
							Name:      "dest-nad",
							Namespace: "dest-ns",
						},
					},
					{
						Source: ref.Ref{
							ID:   "network-3",
							Name: "ns1/network-three",
						},
						Destination: api.DestinationNetwork{
							Type: Ignored,
						},
					},
				},
			},
		}
	})

	Describe("FindNetwork", func() {
		It("should find network by ID", func() {
			pair, found := nm.FindNetwork("network-1")
			Expect(found).To(BeTrue())
			Expect(pair.Source.ID).To(Equal("network-1"))
			Expect(pair.Destination.Type).To(Equal(Pod))
		})

		It("should not find non-existent network", func() {
			_, found := nm.FindNetwork("non-existent")
			Expect(found).To(BeFalse())
		})
	})

	Describe("FindNetworkByType", func() {
		It("should find network by type", func() {
			pair, found := nm.FindNetworkByType("bridge")
			Expect(found).To(BeTrue())
			Expect(pair.Source.Type).To(Equal("bridge"))
			Expect(pair.Source.ID).To(Equal("network-1"))
		})

		It("should not find network with non-existent type", func() {
			_, found := nm.FindNetworkByType("vlan")
			Expect(found).To(BeFalse())
		})
	})

	Describe("FindNetworkByNameAndNamespace", func() {
		It("should find network by namespace and name when source has namespace", func() {
			pair, found := nm.FindNetworkByNameAndNamespace("source-ns", "network-two")
			Expect(found).To(BeTrue())
			Expect(pair.Source.Name).To(Equal("network-two"))
			Expect(pair.Source.Namespace).To(Equal("source-ns"))
		})

		It("should find network by namespace/name format when source has no namespace", func() {
			pair, found := nm.FindNetworkByNameAndNamespace("ns1", "network-three")
			Expect(found).To(BeTrue())
			Expect(pair.Source.Name).To(Equal("ns1/network-three"))
		})

		It("should not find network with wrong namespace", func() {
			_, found := nm.FindNetworkByNameAndNamespace("wrong-ns", "network-two")
			Expect(found).To(BeFalse())
		})

		It("should not find non-existent network", func() {
			_, found := nm.FindNetworkByNameAndNamespace("non-existent", "non-existent")
			Expect(found).To(BeFalse())
		})
	})
})

var _ = Describe("MapPredicate", func() {
	var predicate MapPredicate

	BeforeEach(func() {
		predicate = MapPredicate{}
	})

	Describe("Create", func() {
		It("should return true for create events", func() {
			nm := &api.NetworkMap{
				ObjectMeta: meta.ObjectMeta{
					Name:      "test-network-map",
					Namespace: "default",
				},
			}
			result := predicate.Create(event.TypedCreateEvent[*api.NetworkMap]{Object: nm})
			Expect(result).To(BeTrue())
		})
	})

	Describe("Update", func() {
		It("should return true when generation changed", func() {
			oldNm := &api.NetworkMap{
				ObjectMeta: meta.ObjectMeta{
					Name:       "test-network-map",
					Namespace:  "default",
					Generation: 1,
				},
				Status: api.MapStatus{
					ObservedGeneration: 1,
				},
			}
			newNm := &api.NetworkMap{
				ObjectMeta: meta.ObjectMeta{
					Name:       "test-network-map",
					Namespace:  "default",
					Generation: 2,
				},
				Status: api.MapStatus{
					ObservedGeneration: 1,
				},
			}
			result := predicate.Update(event.TypedUpdateEvent[*api.NetworkMap]{
				ObjectOld: oldNm,
				ObjectNew: newNm,
			})
			Expect(result).To(BeTrue())
		})

		It("should return false when generation not changed", func() {
			oldNm := &api.NetworkMap{
				ObjectMeta: meta.ObjectMeta{
					Name:       "test-network-map",
					Namespace:  "default",
					Generation: 1,
				},
				Status: api.MapStatus{
					ObservedGeneration: 1,
				},
			}
			newNm := &api.NetworkMap{
				ObjectMeta: meta.ObjectMeta{
					Name:       "test-network-map",
					Namespace:  "default",
					Generation: 1,
				},
				Status: api.MapStatus{
					ObservedGeneration: 1,
				},
			}
			result := predicate.Update(event.TypedUpdateEvent[*api.NetworkMap]{
				ObjectOld: oldNm,
				ObjectNew: newNm,
			})
			Expect(result).To(BeFalse())
		})
	})

	Describe("Delete", func() {
		It("should return true for delete events", func() {
			nm := &api.NetworkMap{
				ObjectMeta: meta.ObjectMeta{
					Name:      "test-network-map",
					Namespace: "default",
				},
			}
			result := predicate.Delete(event.TypedDeleteEvent[*api.NetworkMap]{Object: nm})
			Expect(result).To(BeTrue())
		})
	})
})

var _ = Describe("ProviderPredicate", func() {
	var providerPredicate *ProviderPredicate

	BeforeEach(func() {
		providerPredicate = &ProviderPredicate{
			channel: make(chan event.GenericEvent, 10),
		}
	})

	Describe("Create", func() {
		It("should return true when provider is reconciled", func() {
			p := &api.Provider{
				ObjectMeta: meta.ObjectMeta{
					Name:       "test-provider",
					Namespace:  "default",
					Generation: 1,
				},
				Status: api.ProviderStatus{
					ObservedGeneration: 1,
				},
			}
			result := providerPredicate.Create(event.TypedCreateEvent[*api.Provider]{Object: p})
			Expect(result).To(BeTrue())
		})

		It("should return false when provider is not reconciled", func() {
			p := &api.Provider{
				ObjectMeta: meta.ObjectMeta{
					Name:       "test-provider",
					Namespace:  "default",
					Generation: 2,
				},
				Status: api.ProviderStatus{
					ObservedGeneration: 1,
				},
			}
			result := providerPredicate.Create(event.TypedCreateEvent[*api.Provider]{Object: p})
			Expect(result).To(BeFalse())
		})
	})

	Describe("Update", func() {
		It("should return true when provider is reconciled", func() {
			oldP := &api.Provider{
				ObjectMeta: meta.ObjectMeta{
					Name:       "test-provider",
					Namespace:  "default",
					Generation: 1,
				},
			}
			newP := &api.Provider{
				ObjectMeta: meta.ObjectMeta{
					Name:       "test-provider",
					Namespace:  "default",
					Generation: 1,
				},
				Status: api.ProviderStatus{
					ObservedGeneration: 1,
				},
			}
			result := providerPredicate.Update(event.TypedUpdateEvent[*api.Provider]{
				ObjectOld: oldP,
				ObjectNew: newP,
			})
			Expect(result).To(BeTrue())
		})

		It("should return false when provider is not reconciled", func() {
			oldP := &api.Provider{
				ObjectMeta: meta.ObjectMeta{
					Name:       "test-provider",
					Namespace:  "default",
					Generation: 1,
				},
			}
			newP := &api.Provider{
				ObjectMeta: meta.ObjectMeta{
					Name:       "test-provider",
					Namespace:  "default",
					Generation: 2,
				},
				Status: api.ProviderStatus{
					ObservedGeneration: 1,
				},
			}
			result := providerPredicate.Update(event.TypedUpdateEvent[*api.Provider]{
				ObjectOld: oldP,
				ObjectNew: newP,
			})
			Expect(result).To(BeFalse())
		})
	})

	Describe("Delete", func() {
		It("should return true for delete events", func() {
			p := &api.Provider{
				ObjectMeta: meta.ObjectMeta{
					Name:      "test-provider",
					Namespace: "default",
				},
			}
			result := providerPredicate.Delete(event.TypedDeleteEvent[*api.Provider]{Object: p})
			Expect(result).To(BeTrue())
		})
	})

	Describe("Generic", func() {
		It("should return true when provider is reconciled", func() {
			p := &api.Provider{
				ObjectMeta: meta.ObjectMeta{
					Name:       "test-provider",
					Namespace:  "default",
					Generation: 1,
				},
				Status: api.ProviderStatus{
					ObservedGeneration: 1,
				},
			}
			result := providerPredicate.Generic(event.TypedGenericEvent[*api.Provider]{Object: p})
			Expect(result).To(BeTrue())
		})

		It("should return false when provider is not reconciled", func() {
			p := &api.Provider{
				ObjectMeta: meta.ObjectMeta{
					Name:       "test-provider",
					Namespace:  "default",
					Generation: 2,
				},
				Status: api.ProviderStatus{
					ObservedGeneration: 1,
				},
			}
			result := providerPredicate.Generic(event.TypedGenericEvent[*api.Provider]{Object: p})
			Expect(result).To(BeFalse())
		})
	})
})
