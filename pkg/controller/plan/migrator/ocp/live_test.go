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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
				Source: ref.Ref{
					Namespace: "source",
					Name:      "net-attach-def",
				},
				Destination: api.DestinationNetwork{
					Namespace: "target",
					Name:      "net-attach-target",
				},
			}
			// it shouldn't get confused by an NAD in
			// an unrelated namespace that happens to
			// have the same name.
			unrelated := api.NetworkPair{
				Source: ref.Ref{
					Namespace: "unrelated",
					Name:      "net-attach-def",
				},
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
			Expect(target.Spec.Template.Spec.Networks[0].Multus.NetworkName).To(Equal("target/net-attach-target"))
		})

		It("Remaps a namespace-qualified network", func() {
			target = makeVM("source/net-attach-def")
			builder.mapNetworks("source", target)
			Expect(target.Spec.Template.Spec.Networks[0].Multus.NetworkName).To(Equal("target/net-attach-target"))
		})
	})

	Context("mapNetworks for a source network with a namespaced name", func() {
		BeforeEach(func() {
			source := api.NetworkPair{
				Source: ref.Ref{
					Namespace: "",
					Name:      "source/net-attach-def",
				},
				Destination: api.DestinationNetwork{
					Namespace: "target",
					Name:      "net-attach-target",
				},
			}
			// it shouldn't get confused by an NAD in
			// an unrelated namespace that happens to
			// have the same name.
			unrelated := api.NetworkPair{
				Source: ref.Ref{
					Namespace: "unrelated",
					Name:      "net-attach-def",
				},
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
			Expect(target.Spec.Template.Spec.Networks[0].Multus.NetworkName).To(Equal("target/net-attach-target"))
		})

		It("Remaps a namespace-qualified network", func() {
			target = makeVM("source/net-attach-def")
			builder.mapNetworks("source", target)
			Expect(target.Spec.Template.Spec.Networks[0].Multus.NetworkName).To(Equal("target/net-attach-target"))
		})
	})

	Context("targetPvc preserves source access mode and volume mode when storage mapping omits them", func() {
		BeforeEach(func() {
			builder = makeBuilderWithPlan("target-ns")
		})

		It("uses storage mapping AccessMode when set", func() {
			source := &model.PersistentVolumeClaim{
				Object: core.PersistentVolumeClaim{
					Spec: core.PersistentVolumeClaimSpec{
						Resources: core.VolumeResourceRequirements{
							Requests: core.ResourceList{core.ResourceStorage: resource.MustParse("10Gi")},
						},
						AccessModes: []core.PersistentVolumeAccessMode{core.ReadWriteOnce},
					},
				},
			}
			source.Name = "my-pvc"
			storage := api.DestinationStorage{
				StorageClass: "target-sc",
				AccessMode:   core.ReadWriteMany,
			}
			pvc := builder.targetPvc(source, storage)
			Expect(pvc.Spec.AccessModes).To(ConsistOf(core.ReadWriteMany))
			Expect(pvc.Namespace).To(Equal("target-ns"))
			Expect(pvc.Name).To(Equal("my-pvc"))
		})

		It("preserves source AccessModes when storage mapping AccessMode is empty", func() {
			source := &model.PersistentVolumeClaim{
				Object: core.PersistentVolumeClaim{
					Spec: core.PersistentVolumeClaimSpec{
						Resources: core.VolumeResourceRequirements{
							Requests: core.ResourceList{core.ResourceStorage: resource.MustParse("10Gi")},
						},
						AccessModes: []core.PersistentVolumeAccessMode{core.ReadWriteOnce},
					},
				},
			}
			source.Name = "my-pvc"
			storage := api.DestinationStorage{StorageClass: "target-sc"}
			pvc := builder.targetPvc(source, storage)
			Expect(pvc.Spec.AccessModes).To(ConsistOf(core.ReadWriteOnce))
		})

		It("preserves source VolumeMode when storage mapping VolumeMode is empty", func() {
			blockMode := core.PersistentVolumeBlock
			source := &model.PersistentVolumeClaim{
				Object: core.PersistentVolumeClaim{
					Spec: core.PersistentVolumeClaimSpec{
						Resources: core.VolumeResourceRequirements{
							Requests: core.ResourceList{core.ResourceStorage: resource.MustParse("10Gi")},
						},
						VolumeMode: &blockMode,
					},
				},
			}
			source.Name = "my-pvc"
			storage := api.DestinationStorage{StorageClass: "target-sc"}
			pvc := builder.targetPvc(source, storage)
			Expect(pvc.Spec.VolumeMode).ToNot(BeNil())
			Expect(*pvc.Spec.VolumeMode).To(Equal(core.PersistentVolumeBlock))
		})
	})

	Context("targetDataVolume preserves source access mode and volume mode when storage mapping omits them", func() {
		BeforeEach(func() {
			builder = makeBuilderWithPlan("target-ns")
		})

		It("uses storage mapping AccessMode when set", func() {
			sourceDV := &model.DataVolume{
				Object: cdi.DataVolume{
					ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{}, Labels: map[string]string{}},
					Spec:       cdi.DataVolumeSpec{},
				},
			}
			sourceDV.Name = "my-dv"
			sourcePVC := &model.PersistentVolumeClaim{
				Object: core.PersistentVolumeClaim{
					Spec: core.PersistentVolumeClaimSpec{
						Resources: core.VolumeResourceRequirements{
							Requests: core.ResourceList{core.ResourceStorage: resource.MustParse("10Gi")},
						},
					},
				},
			}
			storage := api.DestinationStorage{
				StorageClass: "target-sc",
				AccessMode:   core.ReadWriteMany,
			}
			dv := builder.targetDataVolume(sourceDV, sourcePVC, storage)
			Expect(dv.Spec.Storage.AccessModes).To(ConsistOf(core.ReadWriteMany))
		})

		It("preserves source DataVolume Storage.AccessModes when storage mapping AccessMode is empty", func() {
			sourceDV := &model.DataVolume{
				Object: cdi.DataVolume{
					ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{}, Labels: map[string]string{}},
					Spec: cdi.DataVolumeSpec{
						Storage: &cdi.StorageSpec{
							AccessModes: []core.PersistentVolumeAccessMode{core.ReadWriteOnce},
						},
					},
				},
			}
			sourceDV.Name = "my-dv"
			sourcePVC := &model.PersistentVolumeClaim{
				Object: core.PersistentVolumeClaim{
					Spec: core.PersistentVolumeClaimSpec{
						Resources: core.VolumeResourceRequirements{
							Requests: core.ResourceList{core.ResourceStorage: resource.MustParse("10Gi")},
						},
					},
				},
			}
			storage := api.DestinationStorage{StorageClass: "target-sc"}
			dv := builder.targetDataVolume(sourceDV, sourcePVC, storage)
			Expect(dv.Spec.Storage.AccessModes).To(ConsistOf(core.ReadWriteOnce))
		})

		It("preserves source PVC AccessModes when storage mapping AccessMode is empty and DV has no Storage.AccessModes", func() {
			sourceDV := &model.DataVolume{
				Object: cdi.DataVolume{
					ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{}, Labels: map[string]string{}},
					Spec:       cdi.DataVolumeSpec{},
				},
			}
			sourceDV.Name = "my-dv"
			sourcePVC := &model.PersistentVolumeClaim{
				Object: core.PersistentVolumeClaim{
					Spec: core.PersistentVolumeClaimSpec{
						Resources: core.VolumeResourceRequirements{
							Requests: core.ResourceList{core.ResourceStorage: resource.MustParse("10Gi")},
						},
						AccessModes: []core.PersistentVolumeAccessMode{core.ReadWriteOnce},
					},
				},
			}
			storage := api.DestinationStorage{StorageClass: "target-sc"}
			dv := builder.targetDataVolume(sourceDV, sourcePVC, storage)
			Expect(dv.Spec.Storage.AccessModes).To(ConsistOf(core.ReadWriteOnce))
		})

		It("preserves source PVC VolumeMode when storage mapping VolumeMode is empty", func() {
			blockMode := core.PersistentVolumeBlock
			sourceDV := &model.DataVolume{
				Object: cdi.DataVolume{
					ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{}, Labels: map[string]string{}},
					Spec:       cdi.DataVolumeSpec{},
				},
			}
			sourceDV.Name = "my-dv"
			sourcePVC := &model.PersistentVolumeClaim{
				Object: core.PersistentVolumeClaim{
					Spec: core.PersistentVolumeClaimSpec{
						Resources: core.VolumeResourceRequirements{
							Requests: core.ResourceList{core.ResourceStorage: resource.MustParse("10Gi")},
						},
						VolumeMode: &blockMode,
					},
				},
			}
			storage := api.DestinationStorage{StorageClass: "target-sc"}
			dv := builder.targetDataVolume(sourceDV, sourcePVC, storage)
			Expect(dv.Spec.Storage.VolumeMode).ToNot(BeNil())
			Expect(*dv.Spec.Storage.VolumeMode).To(Equal(core.PersistentVolumeBlock))
		})
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
	return b
}

func makeBuilderWithPlan(targetNamespace string) *Builder {
	b := &Builder{}
	b.Context = &plancontext.Context{}
	b.Context.Plan = &api.Plan{
		Spec: api.PlanSpec{
			TargetNamespace: targetNamespace,
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
