package ocp

import (
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/ocp"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	cnv "kubevirt.io/api/core/v1"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

func TestOCPLiveMigrator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "OCP Live Migrator Suite")
}

var _ = Describe("Builder", func() {
	var (
		builder *Builder
		target  *cnv.VirtualMachine
	)

	Context("mapNetworks for a source network with a separately specified namespace", func() {
		BeforeEach(func() {
			source := api.NetworkPair{
				Source: api.NetworkSourceRef{Ref: ref.Ref{
					Namespace: "source",
					Name:      "net-attach-def",
				}},
				Destination: api.DestinationNetwork{
					Namespace: "target",
					Name:      "net-attach-target",
				},
			}
			// it shouldn't get confused by an NAD in
			// an unrelated namespace that happens to
			// have the same name.
			unrelated := api.NetworkPair{
				Source: api.NetworkSourceRef{Ref: ref.Ref{
					Namespace: "unrelated",
					Name:      "net-attach-def",
				}},
				Destination: api.DestinationNetwork{
					Namespace: "target",
					Name:      "unrelated",
				},
			}
			builder = makeBuilder(source, unrelated)
		})

		It("Remaps an unqualified network", func() {
			target = makeVM("net-attach-def")
			builder.mapNetworks("source", target)
			// NAD and VM are in same namespace "target", so use unqualified name
			Expect(target.Spec.Template.Spec.Networks[0].Multus.NetworkName).To(Equal("net-attach-target"))
		})

		It("Remaps a namespace-qualified network", func() {
			target = makeVM("source/net-attach-def")
			builder.mapNetworks("source", target)
			// NAD and VM are in same namespace "target", so use unqualified name
			Expect(target.Spec.Template.Spec.Networks[0].Multus.NetworkName).To(Equal("net-attach-target"))
		})
	})

	Context("mapNetworks for a source network with a namespaced name", func() {
		BeforeEach(func() {
			source := api.NetworkPair{
				Source: api.NetworkSourceRef{Ref: ref.Ref{
					Namespace: "",
					Name:      "source/net-attach-def",
				}},
				Destination: api.DestinationNetwork{
					Namespace: "target",
					Name:      "net-attach-target",
				},
			}
			// it shouldn't get confused by an NAD in
			// an unrelated namespace that happens to
			// have the same name.
			unrelated := api.NetworkPair{
				Source: api.NetworkSourceRef{Ref: ref.Ref{
					Namespace: "unrelated",
					Name:      "net-attach-def",
				}},
				Destination: api.DestinationNetwork{
					Namespace: "target",
					Name:      "unrelated",
				},
			}
			builder = makeBuilder(source, unrelated)
		})

		It("Remaps an unqualified network", func() {
			target = makeVM("net-attach-def")
			builder.mapNetworks("source", target)
			// NAD and VM are in same namespace "target", so use unqualified name
			Expect(target.Spec.Template.Spec.Networks[0].Multus.NetworkName).To(Equal("net-attach-target"))
		})

		It("Remaps a namespace-qualified network", func() {
			target = makeVM("source/net-attach-def")
			builder.mapNetworks("source", target)
			// NAD and VM are in same namespace "target", so use unqualified name
			Expect(target.Spec.Template.Spec.Networks[0].Multus.NetworkName).To(Equal("net-attach-target"))
		})
	})

	Context("mapNetworks with different namespaces", func() {
		BeforeEach(func() {
			source := api.NetworkPair{
				Source: api.NetworkSourceRef{Ref: ref.Ref{
					Namespace: "source",
					Name:      "net-attach-def",
				}},
				Destination: api.DestinationNetwork{
					Namespace: "different", // NAD in different namespace than target VM
					Name:      "net-attach-target",
				},
			}
			builder = makeBuilder(source)
		})

		It("Uses qualified name when NAD and VM are in different namespaces", func() {
			target = makeVM("net-attach-def")
			builder.mapNetworks("source", target)
			// NAD is in "different" namespace, VM is in "target" - use qualified name
			Expect(target.Spec.Template.Spec.Networks[0].Multus.NetworkName).To(Equal("different/net-attach-target"))
		})
	})
})

var _ = Describe("target PVC", func() {
	var builder *Builder
	var sourcePvc *model.PersistentVolumeClaim
	var storage api.DestinationStorage

	BeforeEach(func() {
		builder = &Builder{}
		builder.Context = &plancontext.Context{}
		builder.Plan = &api.Plan{}
		builder.Plan.Spec = api.PlanSpec{}
		builder.Plan.Spec.TargetNamespace = "test"
		sourcePvc = &model.PersistentVolumeClaim{}
		storage = api.DestinationStorage{}
	})

	It("should set the access mode from the storage map if available", func() {
		storage.AccessMode = core.ReadWriteOnce
		pvc := builder.targetPvc(sourcePvc, storage)
		Expect(pvc.Spec.AccessModes[0]).To(Equal(core.ReadWriteOnce))
	})

	It("should copy the accessmode from the source PVC if not in the storage map", func() {
		sourcePvc.Object.Spec.AccessModes = []core.PersistentVolumeAccessMode{core.ReadWriteMany}
		pvc := builder.targetPvc(sourcePvc, storage)
		Expect(pvc.Spec.AccessModes[0]).To(Equal(core.ReadWriteMany))
	})

})

var _ = Describe("target DataVolume", func() {
	var builder *Builder
	var storage api.DestinationStorage

	BeforeEach(func() {
		builder = &Builder{}
		builder.Context = &plancontext.Context{}
		builder.Plan = &api.Plan{Spec: api.PlanSpec{TargetNamespace: "test"}}
		storage = api.DestinationStorage{}
	})

	makeDV := func(storage *cdi.StorageSpec) *model.DataVolume {
		return &model.DataVolume{
			Object: cdi.DataVolume{
				ObjectMeta: meta.ObjectMeta{Annotations: map[string]string{}},
				Spec: cdi.DataVolumeSpec{
					Storage: storage,
				},
			},
		}
	}

	makeBlockPvc := func(request, capacity string) *model.PersistentVolumeClaim {
		blockMode := core.PersistentVolumeBlock
		return &model.PersistentVolumeClaim{
			Object: core.PersistentVolumeClaim{
				Spec: core.PersistentVolumeClaimSpec{
					VolumeMode: &blockMode,
					Resources: core.VolumeResourceRequirements{
						Requests: core.ResourceList{
							core.ResourceStorage: resource.MustParse(request),
						},
					},
				},
				Status: core.PersistentVolumeClaimStatus{
					Capacity: core.ResourceList{
						core.ResourceStorage: resource.MustParse(capacity),
					},
				},
			},
		}
	}

	It("bumps the target size when the source DV request is smaller than the source capacity (MTV-7343)", func() {
		// Source DV asked for the request size, but the backing PV was
		// provisioned larger (e.g. PowerFlex rounding).
		source := makeDV(&cdi.StorageSpec{
			Resources: core.VolumeResourceRequirements{
				Requests: core.ResourceList{
					core.ResourceStorage: resource.MustParse("111Gi"),
				},
			},
		})
		pvc := makeBlockPvc("111Gi", "112Gi")
		dv := builder.targetDataVolume(source, pvc, storage)
		Expect(dv.Spec.Storage.Resources.Requests.Storage().Cmp(resource.MustParse("112Gi"))).To(Equal(0))
	})

	It("keeps the source DV request when it is at least the source capacity", func() {
		source := makeDV(&cdi.StorageSpec{
			Resources: core.VolumeResourceRequirements{
				Requests: core.ResourceList{
					core.ResourceStorage: resource.MustParse("200Gi"),
				},
			},
		})
		pvc := makeBlockPvc("111Gi", "112Gi")
		dv := builder.targetDataVolume(source, pvc, storage)
		Expect(dv.Spec.Storage.Resources.Requests.Storage().Cmp(resource.MustParse("200Gi"))).To(Equal(0))
	})

	It("uses the source capacity when the DV has no storage spec", func() {
		source := makeDV(nil)
		pvc := makeBlockPvc("111Gi", "112Gi")
		dv := builder.targetDataVolume(source, pvc, storage)
		Expect(dv.Spec.Storage.Resources.Requests.Storage().Cmp(resource.MustParse("112Gi"))).To(Equal(0))
	})

	It("raises the limit to match the bumped request (MTV-7343)", func() {
		// A limit copied from the source that is smaller than the
		// raised request would be rejected by admission.
		source := makeDV(&cdi.StorageSpec{
			Resources: core.VolumeResourceRequirements{
				Requests: core.ResourceList{
					core.ResourceStorage: resource.MustParse("111Gi"),
				},
				Limits: core.ResourceList{
					core.ResourceStorage: resource.MustParse("111Gi"),
				},
			},
		})
		pvc := makeBlockPvc("111Gi", "112Gi")
		dv := builder.targetDataVolume(source, pvc, storage)
		Expect(dv.Spec.Storage.Resources.Requests.Storage().Cmp(resource.MustParse("112Gi"))).To(Equal(0))
		Expect(dv.Spec.Storage.Resources.Limits.Storage().Cmp(resource.MustParse("112Gi"))).To(Equal(0))
	})

	It("keeps a limit that is at least the target size", func() {
		source := makeDV(&cdi.StorageSpec{
			Resources: core.VolumeResourceRequirements{
				Requests: core.ResourceList{
					core.ResourceStorage: resource.MustParse("111Gi"),
				},
				Limits: core.ResourceList{
					core.ResourceStorage: resource.MustParse("200Gi"),
				},
			},
		})
		pvc := makeBlockPvc("111Gi", "112Gi")
		dv := builder.targetDataVolume(source, pvc, storage)
		Expect(dv.Spec.Storage.Resources.Limits.Storage().Cmp(resource.MustParse("200Gi"))).To(Equal(0))
	})
})

func makeBuilder(networkPairs ...api.NetworkPair) *Builder {
	b := &Builder{}
	b.Context = &plancontext.Context{}
	b.Map.Network = &api.NetworkMap{
		Spec: api.NetworkMapSpec{
			Map: networkPairs,
		},
	}
	// Set up a Plan with target namespace for testing
	b.Plan = &api.Plan{
		Spec: api.PlanSpec{
			TargetNamespace: "target",
		},
	}
	return b
}

func makeVM(networkName string) *cnv.VirtualMachine {
	return &cnv.VirtualMachine{
		Spec: cnv.VirtualMachineSpec{
			Template: &cnv.VirtualMachineInstanceTemplateSpec{
				Spec: cnv.VirtualMachineInstanceSpec{
					Networks: []cnv.Network{
						// Multus network that should be remapped
						{
							Name: "net-multus",
							NetworkSource: cnv.NetworkSource{
								Multus: &cnv.MultusNetwork{NetworkName: networkName},
							},
						},
					},
				},
			},
		},
	}
}
