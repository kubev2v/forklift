package openstack

import (
	v1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/openstack"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var populatorLog = logging.WithName("openstack-populator-test")

// newPopulatorBuilder builds a Builder wired with a fake k8s client and a mock
// inventory, ready to exercise PopulatorVolumes end-to-end.
func newPopulatorBuilder(workload *model.Workload, images map[string]*model.Image) *Builder {
	scheme := runtime.NewScheme()
	Expect(core.AddToScheme(scheme)).To(Succeed())
	Expect(cdi.AddToScheme(scheme)).To(Succeed())
	Expect(v1beta1.SchemeBuilder.AddToScheme(scheme)).To(Succeed())

	storageProfile := &cdi.StorageProfile{
		ObjectMeta: metav1.ObjectMeta{
			Name: "sc-1",
		},
		Status: cdi.StorageProfileStatus{
			ClaimPropertySets: []cdi.ClaimPropertySet{
				{
					AccessModes: []core.PersistentVolumeAccessMode{core.ReadWriteOnce},
					VolumeMode:  ptr.To(core.PersistentVolumeFilesystem),
				},
			},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(storageProfile).
		Build()

	plan := &v1beta1.Plan{
		ObjectMeta: metav1.ObjectMeta{Name: "test-plan", Namespace: "test-ns"},
		Spec: v1beta1.PlanSpec{
			TargetNamespace: "test-ns",
			// Deterministic naming (non-generateName) is what exposes the disk-index
			// collision bug when a disk lags behind others in becoming ready.
			PVCNameTemplate:                "{{.TargetVmName}}-disk-{{.DiskIndex}}",
			PVCNameTemplateUseGenerateName: ptr.To(false),
		},
	}
	plan.Map.Storage = &v1beta1.StorageMap{
		Spec: v1beta1.StorageMapSpec{
			Map: []v1beta1.StoragePair{
				{
					Source:      ref.Ref{ID: "voltype-1"},
					Destination: v1beta1.DestinationStorage{StorageClass: "sc-1"},
				},
				{
					// Used for image-based VMs / VM-snapshot images (no volume type).
					Source:      ref.Ref{Name: v1beta1.GlanceSource},
					Destination: v1beta1.DestinationStorage{StorageClass: "sc-1"},
				},
			},
		},
	}

	inventory := &mockOpenstackInventory{
		workloads: map[string]*model.Workload{workload.ID: workload},
		networks:  map[string]*model.Network{},
		images:    images,
	}

	ctx := &plancontext.Context{
		Client: cl,
		Plan:   plan,
		Migration: &v1beta1.Migration{
			ObjectMeta: metav1.ObjectMeta{UID: "migration-1"},
		},
		Source: plancontext.Source{
			Provider:  &v1beta1.Provider{},
			Inventory: inventory,
		},
		Destination: plancontext.Destination{
			Client: cl,
		},
		Log: populatorLog,
	}
	ctx.Map.Storage = plan.Map.Storage

	return &Builder{Context: ctx}
}

var _ = Describe("OpenStack builder PopulatorVolumes disk index stability", func() {
	It("keeps each volume's disk index stable regardless of which other volumes are ready", func() {
		vmRef := ref.Ref{ID: "vm-1", Name: "myvm"}

		workload := &model.Workload{
			XVM: model.XVM{
				VM: model.VM{
					VM1: model.VM1{
						VM0: model.VM0{ID: "vm-1", Name: "myvm"},
					},
				},
				Volumes: []model.Volume{
					{Resource: model.Resource{ID: "vol-0"}, VolumeType: "voltype-1"},
					{Resource: model.Resource{ID: "vol-1"}, VolumeType: "voltype-1"},
					{Resource: model.Resource{ID: "vol-2"}, VolumeType: "voltype-1"},
				},
				VolumeTypes: []model.VolumeType{
					{Resource: model.Resource{ID: "voltype-1", Name: "voltype-1"}},
				},
			},
		}

		builder := newPopulatorBuilder(workload, nil)

		imageForVolume := func(volumeID string) *model.Image {
			return &model.Image{
				Resource:    model.Resource{ID: "image-" + volumeID, Name: "image-" + volumeID},
				Status:      string(ImageStatusActive),
				DiskFormat:  "raw",
				VirtualSize: 1024,
				Properties: map[string]interface{}{
					forkliftPropertyOriginalVolumeID: volumeID,
				},
			}
		}

		// vol-1's image has not been created yet (still snapshotting/uploading),
		// so it must be entirely absent from the inventory at this point in time.
		images := map[string]*model.Image{
			getImageFromVolumeName(builder.Context, workload.ID, "vol-0"): imageForVolume("vol-0"),
			getImageFromVolumeName(builder.Context, workload.ID, "vol-2"): imageForVolume("vol-2"),
		}
		builder.Source.Inventory.(*mockOpenstackInventory).images = images

		annotations := map[string]string{}
		pvcs, err := builder.PopulatorVolumes(vmRef, annotations, "secret")
		Expect(err).NotTo(HaveOccurred())
		Expect(pvcs).To(HaveLen(2))

		names := make([]string, 0, len(pvcs))
		for _, pvc := range pvcs {
			names = append(names, pvc.Name)
		}

		// vol-0 is the first volume -> disk index 0.
		// vol-2 is the third volume -> disk index 2, even though vol-1 (index 1)
		// is not ready yet and is skipped entirely.
		Expect(names).To(ConsistOf("myvm-disk-0", "myvm-disk-2"))
	})

	It("places the VM-snapshot image after all volume disks so it never collides with a volume's disk index", func() {
		vmRef := ref.Ref{ID: "vm-2", Name: "myvm2"}

		workload := &model.Workload{
			XVM: model.XVM{
				VM: model.VM{
					VM1: model.VM1{
						VM0:     model.VM0{ID: "vm-2", Name: "myvm2"},
						ImageID: "orig-image-id",
					},
				},
				Volumes: []model.Volume{
					{Resource: model.Resource{ID: "vol-a"}, VolumeType: "voltype-1"},
					{Resource: model.Resource{ID: "vol-b"}, VolumeType: "voltype-1"},
				},
				VolumeTypes: []model.VolumeType{
					{Resource: model.Resource{ID: "voltype-1", Name: "voltype-1"}},
				},
			},
		}

		builder := newPopulatorBuilder(workload, nil)

		imageForVolume := func(volumeID string) *model.Image {
			return &model.Image{
				Resource:    model.Resource{ID: "image-" + volumeID, Name: "image-" + volumeID},
				Status:      string(ImageStatusActive),
				DiskFormat:  "raw",
				VirtualSize: 1024,
				Properties: map[string]interface{}{
					forkliftPropertyOriginalVolumeID: volumeID,
				},
			}
		}

		snapshotImage := &model.Image{
			Resource:    model.Resource{ID: "snapshot-image", Name: "snapshot-image"},
			Status:      string(ImageStatusActive),
			DiskFormat:  "raw",
			VirtualSize: 2048,
			Properties: map[string]interface{}{
				forkliftPropertyOriginalImageID: workload.ImageID,
			},
		}

		images := map[string]*model.Image{
			getImageFromVolumeName(builder.Context, workload.ID, "vol-a"): imageForVolume("vol-a"),
			getImageFromVolumeName(builder.Context, workload.ID, "vol-b"): imageForVolume("vol-b"),
			getVmSnapshotName(builder.Context, workload.ID):               snapshotImage,
		}
		builder.Source.Inventory.(*mockOpenstackInventory).images = images

		annotations := map[string]string{}
		pvcs, err := builder.PopulatorVolumes(vmRef, annotations, "secret")
		Expect(err).NotTo(HaveOccurred())
		Expect(pvcs).To(HaveLen(3))

		names := make([]string, 0, len(pvcs))
		var snapshotPvcName string
		for _, pvc := range pvcs {
			names = append(names, pvc.Name)
			if pvc.Labels["imageID"] == snapshotImage.ID {
				snapshotPvcName = pvc.Name
			}
		}

		// vol-a -> disk index 0, vol-b -> disk index 1, snapshot -> disk index
		// len(workload.Volumes) == 2. All three names must be distinct.
		Expect(names).To(ConsistOf("myvm2-disk-0", "myvm2-disk-1", "myvm2-disk-2"))
		Expect(snapshotPvcName).To(Equal("myvm2-disk-2"))
	})
})
