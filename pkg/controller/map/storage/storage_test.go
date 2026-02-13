// Generated-by: Claude
package storage

import (
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func TestStorage(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Storage Map Suite")
}

var _ = Describe("StorageMap Struct", func() {
	Describe("Status Conditions", func() {
		It("should set and check conditions", func() {
			sm := &api.StorageMap{
				ObjectMeta: meta.ObjectMeta{
					Name:      "test-storage-map",
					Namespace: "default",
				},
			}
			sm.Status.SetCondition(libcnd.Condition{
				Type:     SourceStorageNotValid,
				Status:   True,
				Reason:   NotFound,
				Category: Critical,
				Message:  "Source storage not found.",
			})
			Expect(sm.Status.HasCondition(SourceStorageNotValid)).To(BeTrue())
			Expect(sm.Status.HasCondition(DestinationStorageNotValid)).To(BeFalse())
		})

		It("should check multiple conditions with HasAnyCondition", func() {
			sm := &api.StorageMap{
				ObjectMeta: meta.ObjectMeta{
					Name:      "test-storage-map",
					Namespace: "default",
				},
			}
			sm.Status.SetCondition(libcnd.Condition{
				Type:     SourceStorageNotValid,
				Status:   True,
				Reason:   NotFound,
				Category: Critical,
				Message:  "Source storage not found.",
			})
			sm.Status.SetCondition(libcnd.Condition{
				Type:     DestinationStorageNotValid,
				Status:   True,
				Reason:   NotFound,
				Category: Critical,
				Message:  "Destination storage not found.",
			})
			Expect(sm.Status.HasAnyCondition(SourceStorageNotValid, DestinationStorageNotValid)).To(BeTrue())
		})
	})
})

var _ = Describe("StoragePair", func() {
	Describe("Source Ref NotSet", func() {
		It("should return true when source ref is empty", func() {
			pair := api.StoragePair{
				Source: ref.Ref{},
				Destination: api.DestinationStorage{
					StorageClass: "standard",
				},
			}
			Expect(pair.Source.NotSet()).To(BeTrue())
		})

		It("should return false when source has ID", func() {
			pair := api.StoragePair{
				Source: ref.Ref{ID: "datastore-id"},
				Destination: api.DestinationStorage{
					StorageClass: "standard",
				},
			}
			Expect(pair.Source.NotSet()).To(BeFalse())
		})

		It("should return false when source has Name", func() {
			pair := api.StoragePair{
				Source: ref.Ref{Name: "datastore-name"},
				Destination: api.DestinationStorage{
					StorageClass: "standard",
				},
			}
			Expect(pair.Source.NotSet()).To(BeFalse())
		})
	})
})

var _ = Describe("StorageMap Methods", func() {
	var sm *api.StorageMap

	BeforeEach(func() {
		sm = &api.StorageMap{
			ObjectMeta: meta.ObjectMeta{
				Name:      "test-storage-map",
				Namespace: "default",
			},
			Spec: api.StorageMapSpec{
				Map: []api.StoragePair{
					{
						Source: ref.Ref{
							ID:   "datastore-1",
							Name: "ds-one",
						},
						Destination: api.DestinationStorage{
							StorageClass: "standard",
						},
					},
					{
						Source: ref.Ref{
							ID:   "datastore-2",
							Name: "ds-two",
						},
						Destination: api.DestinationStorage{
							StorageClass: "premium-ssd",
							AccessMode:   core.ReadWriteOnce,
							VolumeMode:   core.PersistentVolumeFilesystem,
						},
					},
					{
						Source: ref.Ref{
							ID:   "datastore-3",
							Name: "ds-three",
						},
						Destination: api.DestinationStorage{
							StorageClass: "block-storage",
							AccessMode:   core.ReadWriteMany,
							VolumeMode:   core.PersistentVolumeBlock,
						},
					},
				},
			},
		}
	})

	Describe("FindStorage", func() {
		It("should find storage by ID", func() {
			pair, found := sm.FindStorage("datastore-1")
			Expect(found).To(BeTrue())
			Expect(pair.Source.ID).To(Equal("datastore-1"))
			Expect(pair.Destination.StorageClass).To(Equal("standard"))
		})

		It("should not find non-existent storage", func() {
			_, found := sm.FindStorage("non-existent")
			Expect(found).To(BeFalse())
		})
	})

	Describe("FindStorageByName", func() {
		It("should find storage by name", func() {
			pair, found := sm.FindStorageByName("ds-one")
			Expect(found).To(BeTrue())
			Expect(pair.Source.Name).To(Equal("ds-one"))
			Expect(pair.Source.ID).To(Equal("datastore-1"))
		})

		It("should not find storage with non-existent name", func() {
			_, found := sm.FindStorageByName("non-existent")
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
			sm := &api.StorageMap{
				ObjectMeta: meta.ObjectMeta{
					Name:      "test-storage-map",
					Namespace: "default",
				},
			}
			result := predicate.Create(event.TypedCreateEvent[*api.StorageMap]{Object: sm})
			Expect(result).To(BeTrue())
		})
	})

	Describe("Update", func() {
		It("should return true when generation changed", func() {
			oldSm := &api.StorageMap{
				ObjectMeta: meta.ObjectMeta{
					Name:       "test-storage-map",
					Namespace:  "default",
					Generation: 1,
				},
				Status: api.MapStatus{
					ObservedGeneration: 1,
				},
			}
			newSm := &api.StorageMap{
				ObjectMeta: meta.ObjectMeta{
					Name:       "test-storage-map",
					Namespace:  "default",
					Generation: 2,
				},
				Status: api.MapStatus{
					ObservedGeneration: 1,
				},
			}
			result := predicate.Update(event.TypedUpdateEvent[*api.StorageMap]{
				ObjectOld: oldSm,
				ObjectNew: newSm,
			})
			Expect(result).To(BeTrue())
		})

		It("should return false when generation not changed", func() {
			oldSm := &api.StorageMap{
				ObjectMeta: meta.ObjectMeta{
					Name:       "test-storage-map",
					Namespace:  "default",
					Generation: 1,
				},
				Status: api.MapStatus{
					ObservedGeneration: 1,
				},
			}
			newSm := &api.StorageMap{
				ObjectMeta: meta.ObjectMeta{
					Name:       "test-storage-map",
					Namespace:  "default",
					Generation: 1,
				},
				Status: api.MapStatus{
					ObservedGeneration: 1,
				},
			}
			result := predicate.Update(event.TypedUpdateEvent[*api.StorageMap]{
				ObjectOld: oldSm,
				ObjectNew: newSm,
			})
			Expect(result).To(BeFalse())
		})
	})

	Describe("Delete", func() {
		It("should return true for delete events", func() {
			sm := &api.StorageMap{
				ObjectMeta: meta.ObjectMeta{
					Name:      "test-storage-map",
					Namespace: "default",
				},
			}
			result := predicate.Delete(event.TypedDeleteEvent[*api.StorageMap]{Object: sm})
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

var _ = Describe("StorageVendorProducts", func() {
	It("should return all storage vendor products", func() {
		products := api.StorageVendorProducts()
		Expect(products).To(HaveLen(9))
		Expect(products).To(ContainElement(api.StorageVendorProductFlashSystem))
		Expect(products).To(ContainElement(api.StorageVendorProductVantara))
		Expect(products).To(ContainElement(api.StorageVendorProductOntap))
		Expect(products).To(ContainElement(api.StorageVendorProductPrimera3Par))
		Expect(products).To(ContainElement(api.StorageVendorProductPureFlashArray))
		Expect(products).To(ContainElement(api.StorageVendorProductPowerFlex))
		Expect(products).To(ContainElement(api.StorageVendorProductPowerMax))
		Expect(products).To(ContainElement(api.StorageVendorProductPowerStore))
		Expect(products).To(ContainElement(api.StorageVendorProductInfinibox))
	})
})
