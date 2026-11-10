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
			Expect(target.Spec.Template.Spec.Networks[0].Multus.NetworkName).To(Equal("target/net-attach-target"))
		})

		It("Remaps a namespace-qualified network", func() {
			target = makeVM("source/net-attach-def")
			builder.mapNetworks("source", target)
			Expect(target.Spec.Template.Spec.Networks[0].Multus.NetworkName).To(Equal("target/net-attach-target"))
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

var _ = Describe("revision matchers", func() {
	It("copies a status revision onto the target spec", func() {
		vm := &cnv.VirtualMachine{
			Spec: cnv.VirtualMachineSpec{
				Instancetype: &cnv.InstancetypeMatcher{Name: "u1.medium", Kind: "VirtualMachineClusterInstancetype"},
				Preference:   &cnv.PreferenceMatcher{Name: "pref"},
			},
			Status: cnv.VirtualMachineStatus{
				InstancetypeRef: &cnv.InstancetypeStatusRef{
					ControllerRevisionRef: &cnv.ControllerRevisionRef{Name: "it-status"},
				},
				PreferenceRef: &cnv.InstancetypeStatusRef{
					ControllerRevisionRef: &cnv.ControllerRevisionRef{Name: "pref-status"},
				},
			},
		}

		instancetype := instancetypeMatcher(vm)
		preference := preferenceMatcher(vm)

		Expect(instancetype.Name).To(Equal("u1.medium"))
		Expect(instancetype.Kind).To(Equal("VirtualMachineClusterInstancetype"))
		Expect(instancetype.RevisionName).To(Equal("it-status"))
		Expect(preference.Name).To(Equal("pref"))
		Expect(preference.RevisionName).To(Equal("pref-status"))
		Expect(vm.Spec.Instancetype.RevisionName).To(BeEmpty())
		Expect(vm.Spec.Preference.RevisionName).To(BeEmpty())
	})

	It("keeps the spec revision when status has none", func() {
		vm := &cnv.VirtualMachine{
			Spec: cnv.VirtualMachineSpec{
				Instancetype: &cnv.InstancetypeMatcher{Name: "u1.medium", RevisionName: "it-spec"},
				Preference:   &cnv.PreferenceMatcher{Name: "pref", RevisionName: "pref-spec"},
			},
		}

		Expect(instancetypeMatcher(vm).RevisionName).To(Equal("it-spec"))
		Expect(preferenceMatcher(vm).RevisionName).To(Equal("pref-spec"))
		Expect(vm.Spec.Instancetype.RevisionName).To(Equal("it-spec"))
		Expect(vm.Spec.Preference.RevisionName).To(Equal("pref-spec"))
	})
})

var _ = Describe("revision copy", func() {
	It("copies the revision named by the target matcher spec", func() {
		source, destination := revisionClients(
			revision("workloads", "it-status", `{"from":"workloads"}`),
			revision("other", "it-status", `{"from":"other"}`),
			revision("workloads", "pref-status", `{"from":"preference"}`),
		)
		r := revisionEnsurer(source, destination)
		vm := &planapi.VMStatus{}
		vm.ID = "vm-1"
		vm.Name = "vm"
		vm.Namespace = "workloads"
		target := &cnv.VirtualMachine{
			Spec: cnv.VirtualMachineSpec{
				Instancetype: &cnv.InstancetypeMatcher{RevisionName: "it-status"},
				Preference:   &cnv.PreferenceMatcher{RevisionName: "pref-status"},
			},
		}

		Expect(r.EnsureRevisions(vm, target)).To(Succeed())

		expectRevision(destination, "it-status", `{"from":"workloads"}`)
		expectRevision(destination, "pref-status", `{"from":"preference"}`)
		copied := &appsv1.ControllerRevision{}
		Expect(destination.Get(context.Background(), types.NamespacedName{Namespace: "target", Name: "it-status"}, copied)).To(Succeed())
		Expect(copied.Labels).To(HaveKeyWithValue(plancontext.LabelMigration, "mig-1"))
		Expect(copied.Labels).To(HaveKeyWithValue(plancontext.LabelVM, "vm-1"))
	})

	It("removes revisions this migration created and leaves pre-existing ones", func() {
		source, destination := revisionClients(
			revision("workloads", "it-status", `{"from":"workloads"}`),
		)
		preexisting := revision("target", "already-there", `{"from":"destination"}`)
		Expect(destination.Create(context.Background(), preexisting)).To(Succeed())
		ctx := revisionContext(destination)
		r := &Ensurer{
			Ensurer:      &ensurer.Ensurer{Context: ctx},
			SourceClient: source,
		}
		vm := &planapi.VMStatus{}
		vm.ID = "vm-1"
		vm.Name = "vm"
		vm.Namespace = "workloads"
		target := &cnv.VirtualMachine{
			Spec: cnv.VirtualMachineSpec{
				Instancetype: &cnv.InstancetypeMatcher{RevisionName: "it-status"},
			},
		}
		Expect(r.EnsureRevisions(vm, target)).To(Succeed())

		migrator := &LiveMigrator{Context: ctx}
		Expect(migrator.DeleteControllerRevisions(vm)).To(Succeed())

		err := destination.Get(context.Background(), types.NamespacedName{Namespace: "target", Name: "it-status"}, &appsv1.ControllerRevision{})
		Expect(err).To(HaveOccurred())
		expectRevision(destination, "already-there", `{"from":"destination"}`)
	})
})

var _ = Describe("EnsureSourceVMIM", func() {
	// Cross-namespace live migration on a single cluster: the source and
	// destination providers are the same, so SourceClient and the destination
	// client address the same cluster. The target VMIM already exists in the
	// target namespace carrying identical migration labels. The source VMIM must
	// still be created in the source namespace (MTV-7232).
	It("creates the source VMIM even when a target VMIM with identical labels exists", func() {
		scheme := runtime.NewScheme()
		Expect(cnv.AddToScheme(scheme)).To(Succeed())

		vm := &planapi.VMStatus{}
		vm.ID = "vm-1"
		vm.Name = "vm"
		vm.Namespace = "source"

		ctx := &plancontext.Context{
			Plan: &api.Plan{
				ObjectMeta: meta.ObjectMeta{UID: "plan-1"},
				Spec:       api.PlanSpec{TargetNamespace: "target"},
			},
			Migration: &api.Migration{ObjectMeta: meta.ObjectMeta{UID: "mig-1"}},
			Log:       logging.WithName("test"),
		}
		ctx.Labeler = plancontext.Labeler{Context: ctx}

		// Target VMIM already present in the target namespace with identical labels.
		targetVMIM := &cnv.VirtualMachineInstanceMigration{
			ObjectMeta: meta.ObjectMeta{
				Namespace: "target",
				Name:      "forklift-target",
				Labels:    ctx.Labeler.MigrationVMLabels(vm.Ref),
			},
		}
		cluster := fake.NewClientBuilder().WithScheme(scheme).WithObjects(targetVMIM).Build()
		ctx.Destination = plancontext.Destination{Client: cluster}

		r := &Ensurer{
			Ensurer:      &ensurer.Ensurer{Context: ctx},
			SourceClient: cluster,
		}

		source := &cnv.VirtualMachineInstanceMigration{
			ObjectMeta: meta.ObjectMeta{
				Namespace:    "source",
				GenerateName: "forklift-",
				Labels:       ctx.Labeler.MigrationVMLabels(vm.Ref),
			},
			Spec: cnv.VirtualMachineInstanceMigrationSpec{
				VMIName: "vm",
				SendTo: &cnv.VirtualMachineInstanceMigrationSource{
					MigrationID: "vm-1-mig-1",
					ConnectURL:  "tcp://sync",
				},
			},
		}

		Expect(r.EnsureSourceVMIM(vm, source)).To(Succeed())

		list := &cnv.VirtualMachineInstanceMigrationList{}
		Expect(cluster.List(context.Background(), list, &client.ListOptions{Namespace: "source"})).To(Succeed())
		Expect(list.Items).To(HaveLen(1))
		Expect(list.Items[0].Spec.SendTo).ToNot(BeNil())
	})

	It("does not create a duplicate source VMIM when one already exists", func() {
		scheme := runtime.NewScheme()
		Expect(cnv.AddToScheme(scheme)).To(Succeed())

		vm := &planapi.VMStatus{}
		vm.ID = "vm-1"
		vm.Name = "vm"
		vm.Namespace = "source"

		ctx := &plancontext.Context{
			Plan: &api.Plan{
				ObjectMeta: meta.ObjectMeta{UID: "plan-1"},
				Spec:       api.PlanSpec{TargetNamespace: "target"},
			},
			Migration: &api.Migration{ObjectMeta: meta.ObjectMeta{UID: "mig-1"}},
			Log:       logging.WithName("test"),
		}
		ctx.Labeler = plancontext.Labeler{Context: ctx}

		existing := &cnv.VirtualMachineInstanceMigration{
			ObjectMeta: meta.ObjectMeta{
				Namespace: "source",
				Name:      "forklift-existing",
				Labels:    ctx.Labeler.MigrationVMLabels(vm.Ref),
			},
		}
		cluster := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existing).Build()
		ctx.Destination = plancontext.Destination{Client: cluster}

		r := &Ensurer{
			Ensurer:      &ensurer.Ensurer{Context: ctx},
			SourceClient: cluster,
		}

		source := &cnv.VirtualMachineInstanceMigration{
			ObjectMeta: meta.ObjectMeta{
				Namespace:    "source",
				GenerateName: "forklift-",
				Labels:       ctx.Labeler.MigrationVMLabels(vm.Ref),
			},
		}

		Expect(r.EnsureSourceVMIM(vm, source)).To(Succeed())

		list := &cnv.VirtualMachineInstanceMigrationList{}
		Expect(cluster.List(context.Background(), list, &client.ListOptions{Namespace: "source"})).To(Succeed())
		Expect(list.Items).To(HaveLen(1))
		Expect(list.Items[0].Name).To(Equal("forklift-existing"))
	})
})

func revision(namespace, name, data string) *appsv1.ControllerRevision {
	return &appsv1.ControllerRevision{
		ObjectMeta: meta.ObjectMeta{Namespace: namespace, Name: name},
		Data:       runtime.RawExtension{Raw: []byte(data)},
	}
}

func revisionClients(objects ...client.Object) (client.Client, client.Client) {
	scheme := runtime.NewScheme()
	Expect(appsv1.AddToScheme(scheme)).To(Succeed())
	source := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
	destination := fake.NewClientBuilder().WithScheme(scheme).Build()
	return source, destination
}

func revisionContext(destination client.Client) *plancontext.Context {
	ctx := &plancontext.Context{
		Plan: &api.Plan{
			ObjectMeta: meta.ObjectMeta{UID: "plan-1"},
			Spec:       api.PlanSpec{TargetNamespace: "target"},
		},
		Migration:   &api.Migration{ObjectMeta: meta.ObjectMeta{UID: "mig-1"}},
		Destination: plancontext.Destination{Client: destination},
		Log:         logging.WithName("test"),
	}
	ctx.Labeler = plancontext.Labeler{Context: ctx}
	return ctx
}

func revisionEnsurer(source, destination client.Client) *Ensurer {
	return &Ensurer{
		Ensurer:      &ensurer.Ensurer{Context: revisionContext(destination)},
		SourceClient: source,
	}
}

func expectRevision(destination client.Client, name, data string) {
	got := &appsv1.ControllerRevision{}
	Expect(destination.Get(context.Background(), types.NamespacedName{Namespace: "target", Name: name}, got)).To(Succeed())
	Expect(got.Data.Raw).To(Equal([]byte(data)))
}

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
