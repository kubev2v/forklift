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
	cnv "kubevirt.io/api/core/v1"
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

var _ = Describe("requestSize", func() {
	var builder *Builder

	BeforeEach(func() {
		builder = &Builder{}
	})

	It("returns the requested size for filesystem volumes", func() {
		filesystemMode := core.PersistentVolumeFilesystem
		pvc := &model.PersistentVolumeClaim{
			Object: core.PersistentVolumeClaim{
				Spec: core.PersistentVolumeClaimSpec{
					VolumeMode: &filesystemMode,
					Resources: core.VolumeResourceRequirements{
						Requests: core.ResourceList{
							core.ResourceStorage: resource.MustParse("10Gi"),
						},
					},
				},
				Status: core.PersistentVolumeClaimStatus{
					Capacity: core.ResourceList{
						core.ResourceStorage: resource.MustParse("20Gi"),
					},
				},
			},
		}
		size := builder.requestSize(pvc)
		Expect(size.Equal(resource.MustParse("10Gi"))).To(BeTrue())
	})

	It("returns the requested size when volume mode is nil", func() {
		pvc := &model.PersistentVolumeClaim{
			Object: core.PersistentVolumeClaim{
				Spec: core.PersistentVolumeClaimSpec{
					Resources: core.VolumeResourceRequirements{
						Requests: core.ResourceList{
							core.ResourceStorage: resource.MustParse("10Gi"),
						},
					},
				},
				Status: core.PersistentVolumeClaimStatus{
					Capacity: core.ResourceList{
						core.ResourceStorage: resource.MustParse("20Gi"),
					},
				},
			},
		}
		size := builder.requestSize(pvc)
		Expect(size.Equal(resource.MustParse("10Gi"))).To(BeTrue())
	})

	It("returns the allocated capacity for block volumes", func() {
		blockMode := core.PersistentVolumeBlock
		pvc := &model.PersistentVolumeClaim{
			Object: core.PersistentVolumeClaim{
				Spec: core.PersistentVolumeClaimSpec{
					VolumeMode: &blockMode,
					Resources: core.VolumeResourceRequirements{
						Requests: core.ResourceList{
							core.ResourceStorage: resource.MustParse("10Gi"),
						},
					},
				},
				Status: core.PersistentVolumeClaimStatus{
					Capacity: core.ResourceList{
						core.ResourceStorage: resource.MustParse("16Gi"),
					},
				},
			},
		}
		size := builder.requestSize(pvc)
		Expect(size.Equal(resource.MustParse("16Gi"))).To(BeTrue())
	})

	It("tolerates nil resources", func() {
		blockMode := core.PersistentVolumeBlock
		pvc := &model.PersistentVolumeClaim{
			Object: core.PersistentVolumeClaim{
				Spec: core.PersistentVolumeClaimSpec{
					VolumeMode: &blockMode,
					Resources: core.VolumeResourceRequirements{
						Requests: core.ResourceList{},
					},
				},
				Status: core.PersistentVolumeClaimStatus{
					Capacity: core.ResourceList{},
				},
			},
		}
		size := builder.requestSize(pvc)
		Expect(size.Equal(resource.MustParse("0"))).To(BeTrue())
	})
})

func makeBuilder(networkPairs ...api.NetworkPair) *Builder {
	b := &Builder{}
	b.Context = &plancontext.Context{}
	b.Context.Map.Network = &api.NetworkMap{
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
