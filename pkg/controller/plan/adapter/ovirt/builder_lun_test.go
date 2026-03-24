package ovirt

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/ovirt"
	web "github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	ovirt "github.com/kubev2v/forklift/pkg/controller/provider/web/ovirt"
	libweb "github.com/kubev2v/forklift/pkg/lib/inventory/web"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

//nolint:nilnil
type fakeInventoryClient struct {
	vm *ovirt.Workload
}

func (f *fakeInventoryClient) Finder() web.Finder                { return nil }
func (f *fakeInventoryClient) Get(_ interface{}, _ string) error { return nil }
func (f *fakeInventoryClient) List(_ interface{}, _ ...web.Param) error {
	return nil
}
func (f *fakeInventoryClient) Watch(_ interface{}, _ web.EventHandler) (*libweb.Watch, error) {
	return nil, nil //nolint:nilnil
}
func (f *fakeInventoryClient) Find(resource interface{}, _ ref.Ref) error {
	if w, ok := resource.(*ovirt.Workload); ok && f.vm != nil {
		*w = *f.vm
	}
	return nil
}
func (f *fakeInventoryClient) VM(_ *ref.Ref) (interface{}, error) {
	return nil, nil //nolint:nilnil
}
func (f *fakeInventoryClient) Workload(_ *ref.Ref) (interface{}, error) {
	return nil, nil //nolint:nilnil
}
func (f *fakeInventoryClient) Network(_ *ref.Ref) (interface{}, error) {
	return nil, nil //nolint:nilnil
}
func (f *fakeInventoryClient) Storage(_ *ref.Ref) (interface{}, error) {
	return nil, nil //nolint:nilnil
}
func (f *fakeInventoryClient) Host(_ *ref.Ref) (interface{}, error) {
	return nil, nil //nolint:nilnil
}

func newISCSILunDisk(diskID, storageDomain, address, port, target, lunID string, lunMapping int32, size int64) ovirt.XDiskAttachment {
	return ovirt.XDiskAttachment{
		DiskAttachment: model.DiskAttachment{ID: diskID},
		Disk: ovirt.XDisk{
			Disk: ovirt.Disk{
				Resource:      ovirt.Resource{ID: diskID, Name: diskID},
				StorageDomain: storageDomain,
				StorageType:   "lun",
				Lun: model.Lun{
					LogicalUnits: struct {
						LogicalUnit []model.LogicalUnit `json:"logicalUnit"`
					}{
						LogicalUnit: []model.LogicalUnit{
							{
								LunID:      lunID,
								Address:    address,
								Port:       port,
								Target:     target,
								LunMapping: lunMapping,
								Size:       size,
							},
						},
					},
				},
			},
		},
	}
}

func newStorageMap(pairs ...api.StoragePair) *api.StorageMap {
	return &api.StorageMap{
		Spec: api.StorageMapSpec{
			Map: pairs,
		},
	}
}

func newBuilder(vm *ovirt.Workload, storageMap *api.StorageMap) *Builder {
	log := logging.WithName("builder-lun-test")
	b := &Builder{
		Context: &plancontext.Context{
			Plan: &api.Plan{
				ObjectMeta: meta.ObjectMeta{
					Name:      "test-plan",
					Namespace: "test-ns",
					UID:       types.UID("plan-uid"),
				},
				Spec: api.PlanSpec{
					TargetNamespace: "target-ns",
				},
			},
			Migration: &api.Migration{
				ObjectMeta: meta.ObjectMeta{
					UID: types.UID("migration-uid"),
				},
			},
			Source: plancontext.Source{
				Inventory: &fakeInventoryClient{vm: vm},
			},
			Log: log,
		},
	}
	b.Context.Map.Storage = storageMap
	return b
}

var _ = Describe("LUN storage map tests", func() {
	Describe("LunPersistentVolumes", func() {
		It("should use defaults when LUN disk has no StorageDomain and no disk-ID mapping", func() {
			vm := &ovirt.Workload{}
			vm.DiskAttachments = []ovirt.XDiskAttachment{
				newISCSILunDisk("disk-1", "", "10.0.0.1", "3260", "iqn.target", "lun-001", 0, 10737418240),
			}
			sm := newStorageMap(api.StoragePair{
				Source:      ref.Ref{ID: "sd-1"},
				Destination: api.DestinationStorage{StorageClass: "iscsi-block", VolumeMode: core.PersistentVolumeFilesystem, AccessMode: core.ReadWriteOnce},
			})
			b := newBuilder(vm, sm)

			pvs, err := b.LunPersistentVolumes(ref.Ref{ID: "vm-1"})
			Expect(err).ToNot(HaveOccurred())
			Expect(pvs).To(HaveLen(1))
			Expect(pvs[0].Spec.StorageClassName).To(Equal(""))
			Expect(pvs[0].Spec.AccessModes).To(Equal([]core.PersistentVolumeAccessMode{core.ReadWriteMany}))
			Expect(*pvs[0].Spec.VolumeMode).To(Equal(core.PersistentVolumeBlock))
		})

		It("should apply storage map using disk ID when LUN disk has no StorageDomain", func() {
			vm := &ovirt.Workload{}
			vm.DiskAttachments = []ovirt.XDiskAttachment{
				newISCSILunDisk("disk-1", "", "10.0.0.1", "3260", "iqn.target", "lun-001", 0, 10737418240),
			}
			sm := newStorageMap(api.StoragePair{
				Source:      ref.Ref{ID: "disk-1"},
				Destination: api.DestinationStorage{StorageClass: "fast-storage", VolumeMode: core.PersistentVolumeBlock, AccessMode: core.ReadWriteOnce},
			})
			b := newBuilder(vm, sm)

			pvs, err := b.LunPersistentVolumes(ref.Ref{ID: "vm-1"})
			Expect(err).ToNot(HaveOccurred())
			Expect(pvs).To(HaveLen(1))
			Expect(pvs[0].Spec.StorageClassName).To(Equal("fast-storage"))
			Expect(pvs[0].Spec.AccessModes).To(Equal([]core.PersistentVolumeAccessMode{core.ReadWriteOnce}))
			Expect(*pvs[0].Spec.VolumeMode).To(Equal(core.PersistentVolumeBlock))
		})

		It("should apply storage map when LUN disk has a mapped StorageDomain", func() {
			vm := &ovirt.Workload{}
			vm.DiskAttachments = []ovirt.XDiskAttachment{
				newISCSILunDisk("disk-1", "sd-1", "10.0.0.1", "3260", "iqn.target", "lun-001", 0, 10737418240),
			}
			sm := newStorageMap(api.StoragePair{
				Source:      ref.Ref{ID: "sd-1"},
				Destination: api.DestinationStorage{StorageClass: "iscsi-block", VolumeMode: core.PersistentVolumeFilesystem, AccessMode: core.ReadWriteOnce},
			})
			b := newBuilder(vm, sm)

			pvs, err := b.LunPersistentVolumes(ref.Ref{ID: "vm-1"})
			Expect(err).ToNot(HaveOccurred())
			Expect(pvs).To(HaveLen(1))
			Expect(pvs[0].Spec.StorageClassName).To(Equal("iscsi-block"))
			Expect(pvs[0].Spec.AccessModes).To(Equal([]core.PersistentVolumeAccessMode{core.ReadWriteOnce}))
			Expect(*pvs[0].Spec.VolumeMode).To(Equal(core.PersistentVolumeFilesystem))
		})

		It("should fall back to defaults when StorageDomain is not in the map", func() {
			vm := &ovirt.Workload{}
			vm.DiskAttachments = []ovirt.XDiskAttachment{
				newISCSILunDisk("disk-1", "sd-unknown", "10.0.0.1", "3260", "iqn.target", "lun-001", 0, 10737418240),
			}
			sm := newStorageMap(api.StoragePair{
				Source:      ref.Ref{ID: "sd-1"},
				Destination: api.DestinationStorage{StorageClass: "iscsi-block"},
			})
			b := newBuilder(vm, sm)

			pvs, err := b.LunPersistentVolumes(ref.Ref{ID: "vm-1"})
			Expect(err).ToNot(HaveOccurred())
			Expect(pvs).To(HaveLen(1))
			Expect(pvs[0].Spec.StorageClassName).To(Equal(""))
			Expect(pvs[0].Spec.AccessModes).To(Equal([]core.PersistentVolumeAccessMode{core.ReadWriteMany}))
			Expect(*pvs[0].Spec.VolumeMode).To(Equal(core.PersistentVolumeBlock))
		})

		It("should preserve default volumeMode/accessMode when mapping has only storageClass", func() {
			vm := &ovirt.Workload{}
			vm.DiskAttachments = []ovirt.XDiskAttachment{
				newISCSILunDisk("disk-1", "sd-1", "10.0.0.1", "3260", "iqn.target", "lun-001", 0, 10737418240),
			}
			sm := newStorageMap(api.StoragePair{
				Source:      ref.Ref{ID: "sd-1"},
				Destination: api.DestinationStorage{StorageClass: "iscsi-block"},
			})
			b := newBuilder(vm, sm)

			pvs, err := b.LunPersistentVolumes(ref.Ref{ID: "vm-1"})
			Expect(err).ToNot(HaveOccurred())
			Expect(pvs).To(HaveLen(1))
			Expect(pvs[0].Spec.StorageClassName).To(Equal("iscsi-block"))
			Expect(pvs[0].Spec.AccessModes).To(Equal([]core.PersistentVolumeAccessMode{core.ReadWriteMany}))
			Expect(*pvs[0].Spec.VolumeMode).To(Equal(core.PersistentVolumeBlock))
		})

		It("should set iSCSI PV source fields correctly", func() {
			vm := &ovirt.Workload{}
			vm.DiskAttachments = []ovirt.XDiskAttachment{
				newISCSILunDisk("disk-1", "sd-1", "10.0.0.1", "3260", "iqn.2024-01.com.example:target", "lun-abc", 3, 21474836480),
			}
			sm := newStorageMap(api.StoragePair{
				Source:      ref.Ref{ID: "sd-1"},
				Destination: api.DestinationStorage{StorageClass: "iscsi-sc"},
			})
			b := newBuilder(vm, sm)

			pvs, err := b.LunPersistentVolumes(ref.Ref{ID: "vm-1"})
			Expect(err).ToNot(HaveOccurred())
			Expect(pvs).To(HaveLen(1))
			Expect(pvs[0].Spec.ISCSI).ToNot(BeNil())
			Expect(pvs[0].Spec.ISCSI.TargetPortal).To(Equal("10.0.0.1:3260"))
			Expect(pvs[0].Spec.ISCSI.IQN).To(Equal("iqn.2024-01.com.example:target"))
			Expect(pvs[0].Spec.ISCSI.Lun).To(Equal(int32(3)))
			Expect(pvs[0].Spec.FC).To(BeNil())
		})

		It("should create FC PV source when address is empty", func() {
			vm := &ovirt.Workload{}
			vm.DiskAttachments = []ovirt.XDiskAttachment{
				newISCSILunDisk("disk-1", "", "", "", "", "wwid-fc-001", 0, 10737418240),
			}
			sm := newStorageMap()
			b := newBuilder(vm, sm)

			pvs, err := b.LunPersistentVolumes(ref.Ref{ID: "vm-1"})
			Expect(err).ToNot(HaveOccurred())
			Expect(pvs).To(HaveLen(1))
			Expect(pvs[0].Spec.FC).ToNot(BeNil())
			Expect(pvs[0].Spec.FC.WWIDs).To(Equal([]string{"wwid-fc-001"}))
			Expect(pvs[0].Spec.ISCSI).To(BeNil())
		})

		It("should not panic when storage map is nil (MigrationOnlyConversion)", func() {
			vm := &ovirt.Workload{}
			vm.DiskAttachments = []ovirt.XDiskAttachment{
				newISCSILunDisk("disk-1", "sd-1", "10.0.0.1", "3260", "iqn.target", "lun-001", 0, 10737418240),
			}
			b := newBuilder(vm, nil)

			pvs, err := b.LunPersistentVolumes(ref.Ref{ID: "vm-1"})
			Expect(err).ToNot(HaveOccurred())
			Expect(pvs).To(HaveLen(1))
			Expect(pvs[0].Spec.StorageClassName).To(Equal(""))
			Expect(pvs[0].Spec.AccessModes).To(Equal([]core.PersistentVolumeAccessMode{core.ReadWriteMany}))
			Expect(*pvs[0].Spec.VolumeMode).To(Equal(core.PersistentVolumeBlock))
		})
	})

	Describe("LunPersistentVolumeClaims", func() {
		It("should use defaults when LUN disk has no StorageDomain and no disk-ID mapping", func() {
			vm := &ovirt.Workload{}
			vm.DiskAttachments = []ovirt.XDiskAttachment{
				newISCSILunDisk("disk-1", "", "10.0.0.1", "3260", "iqn.target", "lun-001", 0, 10737418240),
			}
			sm := newStorageMap(api.StoragePair{
				Source:      ref.Ref{ID: "sd-1"},
				Destination: api.DestinationStorage{StorageClass: "iscsi-block"},
			})
			b := newBuilder(vm, sm)

			pvcs, err := b.LunPersistentVolumeClaims(ref.Ref{ID: "vm-1"})
			Expect(err).ToNot(HaveOccurred())
			Expect(pvcs).To(HaveLen(1))
			emptyClass := ""
			Expect(pvcs[0].Spec.StorageClassName).To(Equal(&emptyClass))
			Expect(pvcs[0].Spec.AccessModes).To(Equal([]core.PersistentVolumeAccessMode{core.ReadWriteMany}))
			Expect(*pvcs[0].Spec.VolumeMode).To(Equal(core.PersistentVolumeBlock))
		})

		It("should apply storage map using disk ID when LUN disk has no StorageDomain", func() {
			vm := &ovirt.Workload{}
			vm.DiskAttachments = []ovirt.XDiskAttachment{
				newISCSILunDisk("disk-1", "", "10.0.0.1", "3260", "iqn.target", "lun-001", 0, 10737418240),
			}
			sm := newStorageMap(api.StoragePair{
				Source:      ref.Ref{ID: "disk-1"},
				Destination: api.DestinationStorage{StorageClass: "fast-storage", VolumeMode: core.PersistentVolumeBlock, AccessMode: core.ReadWriteOnce},
			})
			b := newBuilder(vm, sm)

			pvcs, err := b.LunPersistentVolumeClaims(ref.Ref{ID: "vm-1"})
			Expect(err).ToNot(HaveOccurred())
			Expect(pvcs).To(HaveLen(1))
			expectedClass := "fast-storage"
			Expect(pvcs[0].Spec.StorageClassName).To(Equal(&expectedClass))
			Expect(pvcs[0].Spec.AccessModes).To(Equal([]core.PersistentVolumeAccessMode{core.ReadWriteOnce}))
			Expect(*pvcs[0].Spec.VolumeMode).To(Equal(core.PersistentVolumeBlock))
		})

		It("should apply storage map when LUN disk has a mapped StorageDomain", func() {
			vm := &ovirt.Workload{}
			vm.DiskAttachments = []ovirt.XDiskAttachment{
				newISCSILunDisk("disk-1", "sd-1", "10.0.0.1", "3260", "iqn.target", "lun-001", 0, 10737418240),
			}
			sm := newStorageMap(api.StoragePair{
				Source:      ref.Ref{ID: "sd-1"},
				Destination: api.DestinationStorage{StorageClass: "iscsi-block", VolumeMode: core.PersistentVolumeFilesystem, AccessMode: core.ReadWriteOnce},
			})
			b := newBuilder(vm, sm)

			pvcs, err := b.LunPersistentVolumeClaims(ref.Ref{ID: "vm-1"})
			Expect(err).ToNot(HaveOccurred())
			Expect(pvcs).To(HaveLen(1))
			expectedClass := "iscsi-block"
			Expect(pvcs[0].Spec.StorageClassName).To(Equal(&expectedClass))
			Expect(pvcs[0].Spec.AccessModes).To(Equal([]core.PersistentVolumeAccessMode{core.ReadWriteOnce}))
			Expect(*pvcs[0].Spec.VolumeMode).To(Equal(core.PersistentVolumeFilesystem))
		})

		It("should not panic when storage map is nil (MigrationOnlyConversion)", func() {
			vm := &ovirt.Workload{}
			vm.DiskAttachments = []ovirt.XDiskAttachment{
				newISCSILunDisk("disk-1", "sd-1", "10.0.0.1", "3260", "iqn.target", "lun-001", 0, 10737418240),
			}
			b := newBuilder(vm, nil)

			pvcs, err := b.LunPersistentVolumeClaims(ref.Ref{ID: "vm-1"})
			Expect(err).ToNot(HaveOccurred())
			Expect(pvcs).To(HaveLen(1))
			emptyClass := ""
			Expect(pvcs[0].Spec.StorageClassName).To(Equal(&emptyClass))
			Expect(pvcs[0].Spec.AccessModes).To(Equal([]core.PersistentVolumeAccessMode{core.ReadWriteMany}))
			Expect(*pvcs[0].Spec.VolumeMode).To(Equal(core.PersistentVolumeBlock))
		})

		It("should have matching storageClassName between PV and PVC", func() {
			vm := &ovirt.Workload{}
			vm.DiskAttachments = []ovirt.XDiskAttachment{
				newISCSILunDisk("disk-1", "sd-1", "10.0.0.1", "3260", "iqn.target", "lun-001", 0, 10737418240),
			}
			sm := newStorageMap(api.StoragePair{
				Source:      ref.Ref{ID: "sd-1"},
				Destination: api.DestinationStorage{StorageClass: "iscsi-block", VolumeMode: core.PersistentVolumeBlock, AccessMode: core.ReadWriteMany},
			})
			b := newBuilder(vm, sm)

			pvs, err := b.LunPersistentVolumes(ref.Ref{ID: "vm-1"})
			Expect(err).ToNot(HaveOccurred())
			pvcs, err := b.LunPersistentVolumeClaims(ref.Ref{ID: "vm-1"})
			Expect(err).ToNot(HaveOccurred())

			Expect(pvs[0].Spec.StorageClassName).To(Equal(*pvcs[0].Spec.StorageClassName))
			Expect(pvs[0].Spec.AccessModes).To(Equal(pvcs[0].Spec.AccessModes))
			Expect(*pvs[0].Spec.VolumeMode).To(Equal(*pvcs[0].Spec.VolumeMode))
		})
	})

	Describe("StorageMapped validator", func() {
		newValidator := func(vm *ovirt.Workload, storageMap *api.StorageMap) *Validator {
			log := logging.WithName("validator-lun-test")
			p := &api.Plan{
				ObjectMeta: meta.ObjectMeta{Name: "test-plan", Namespace: "test-ns"},
			}
			p.Referenced.Map.Storage = storageMap
			return &Validator{
				Context: &plancontext.Context{
					Plan: p,
					Source: plancontext.Source{
						Inventory: &fakeInventoryClient{vm: vm},
					},
					Log: log,
				},
			}
		}

		It("should pass when LUN disk has empty StorageDomain (skipped)", func() {
			vm := &ovirt.Workload{}
			vm.DiskAttachments = []ovirt.XDiskAttachment{
				newISCSILunDisk("disk-1", "", "10.0.0.1", "3260", "iqn.target", "lun-001", 0, 1024),
			}
			sm := newStorageMap()
			v := newValidator(vm, sm)

			ok, err := v.StorageMapped(ref.Ref{ID: "vm-1"})
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())
		})

		It("should pass when LUN disk StorageDomain is in the map refs", func() {
			vm := &ovirt.Workload{}
			vm.DiskAttachments = []ovirt.XDiskAttachment{
				newISCSILunDisk("disk-1", "sd-1", "10.0.0.1", "3260", "iqn.target", "lun-001", 0, 1024),
			}
			sm := newStorageMap(api.StoragePair{
				Source:      ref.Ref{ID: "sd-1"},
				Destination: api.DestinationStorage{StorageClass: "iscsi-block"},
			})
			sm.Status.Refs.List = []ref.Ref{{ID: "sd-1"}}
			v := newValidator(vm, sm)

			ok, err := v.StorageMapped(ref.Ref{ID: "vm-1"})
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())
		})

		It("should fail when LUN disk StorageDomain is not in the map refs", func() {
			vm := &ovirt.Workload{}
			vm.DiskAttachments = []ovirt.XDiskAttachment{
				newISCSILunDisk("disk-1", "sd-unmapped", "10.0.0.1", "3260", "iqn.target", "lun-001", 0, 1024),
			}
			sm := newStorageMap(api.StoragePair{
				Source:      ref.Ref{ID: "sd-1"},
				Destination: api.DestinationStorage{StorageClass: "iscsi-block"},
			})
			sm.Status.Refs.List = []ref.Ref{{ID: "sd-1"}}
			v := newValidator(vm, sm)

			ok, err := v.StorageMapped(ref.Ref{ID: "vm-1"})
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeFalse())
		})

		It("should pass for image disk with mapped StorageDomain", func() {
			vm := &ovirt.Workload{}
			vm.DiskAttachments = []ovirt.XDiskAttachment{
				{
					DiskAttachment: model.DiskAttachment{ID: "disk-img"},
					Disk: ovirt.XDisk{
						Disk: ovirt.Disk{
							Resource:      ovirt.Resource{ID: "disk-img"},
							StorageDomain: "sd-1",
							StorageType:   "image",
						},
					},
				},
			}
			sm := newStorageMap(api.StoragePair{
				Source:      ref.Ref{ID: "sd-1"},
				Destination: api.DestinationStorage{StorageClass: "ceph-rbd"},
			})
			sm.Status.Refs.List = []ref.Ref{{ID: "sd-1"}}
			v := newValidator(vm, sm)

			ok, err := v.StorageMapped(ref.Ref{ID: "vm-1"})
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())
		})

		It("should return false when storage map is nil", func() {
			vm := &ovirt.Workload{}
			vm.DiskAttachments = []ovirt.XDiskAttachment{
				newISCSILunDisk("disk-1", "sd-1", "10.0.0.1", "3260", "iqn.target", "lun-001", 0, 1024),
			}
			v := newValidator(vm, nil)

			ok, err := v.StorageMapped(ref.Ref{ID: "vm-1"})
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeFalse())
		})

		It("should fail for non-LUN disk with empty StorageDomain (data integrity edge case)", func() {
			vm := &ovirt.Workload{}
			vm.DiskAttachments = []ovirt.XDiskAttachment{
				{
					DiskAttachment: model.DiskAttachment{ID: "disk-img-broken"},
					Disk: ovirt.XDisk{
						Disk: ovirt.Disk{
							Resource:      ovirt.Resource{ID: "disk-img-broken"},
							StorageDomain: "",
							StorageType:   "image",
						},
					},
				},
			}
			sm := newStorageMap()
			v := newValidator(vm, sm)

			ok, err := v.StorageMapped(ref.Ref{ID: "vm-1"})
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeFalse())
		})

		It("should pass with mixed LUN (no SD) and image (mapped) disks", func() {
			vm := &ovirt.Workload{}
			vm.DiskAttachments = []ovirt.XDiskAttachment{
				newISCSILunDisk("disk-lun", "", "10.0.0.1", "3260", "iqn.target", "lun-001", 0, 1024),
				{
					DiskAttachment: model.DiskAttachment{ID: "disk-img"},
					Disk: ovirt.XDisk{
						Disk: ovirt.Disk{
							Resource:      ovirt.Resource{ID: "disk-img"},
							StorageDomain: "sd-1",
							StorageType:   "image",
						},
					},
				},
			}
			sm := newStorageMap(api.StoragePair{
				Source:      ref.Ref{ID: "sd-1"},
				Destination: api.DestinationStorage{StorageClass: "ceph-rbd"},
			})
			sm.Status.Refs.List = []ref.Ref{{ID: "sd-1"}}
			v := newValidator(vm, sm)

			ok, err := v.StorageMapped(ref.Ref{ID: "vm-1"})
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())
		})
	})
})

var _ = Describe("resolveLunStorageSettings", func() {
	It("should return defaults when both storageDomainID and diskID are empty", func() {
		sm := newStorageMap(api.StoragePair{
			Source:      ref.Ref{ID: "sd-1"},
			Destination: api.DestinationStorage{StorageClass: "iscsi-block", VolumeMode: core.PersistentVolumeFilesystem, AccessMode: core.ReadWriteOnce},
		})
		b := newBuilder(&ovirt.Workload{}, sm)

		s := b.resolveLunStorageSettings("", "")
		Expect(s.storageClassName).To(Equal(""))
		Expect(s.volumeMode).To(Equal(core.PersistentVolumeBlock))
		Expect(s.accessMode).To(Equal(core.ReadWriteMany))
	})

	It("should return defaults when storage map is nil", func() {
		b := newBuilder(&ovirt.Workload{}, nil)

		s := b.resolveLunStorageSettings("sd-1", "disk-1")
		Expect(s.storageClassName).To(Equal(""))
		Expect(s.volumeMode).To(Equal(core.PersistentVolumeBlock))
		Expect(s.accessMode).To(Equal(core.ReadWriteMany))
	})

	It("should return defaults when storageDomainID is not in the map", func() {
		sm := newStorageMap(api.StoragePair{
			Source:      ref.Ref{ID: "sd-1"},
			Destination: api.DestinationStorage{StorageClass: "iscsi-block"},
		})
		b := newBuilder(&ovirt.Workload{}, sm)

		s := b.resolveLunStorageSettings("sd-unknown", "disk-unknown")
		Expect(s.storageClassName).To(Equal(""))
		Expect(s.volumeMode).To(Equal(core.PersistentVolumeBlock))
		Expect(s.accessMode).To(Equal(core.ReadWriteMany))
	})

	It("should apply all mapped settings when fully specified", func() {
		sm := newStorageMap(api.StoragePair{
			Source:      ref.Ref{ID: "sd-1"},
			Destination: api.DestinationStorage{StorageClass: "iscsi-block", VolumeMode: core.PersistentVolumeFilesystem, AccessMode: core.ReadWriteOnce},
		})
		b := newBuilder(&ovirt.Workload{}, sm)

		s := b.resolveLunStorageSettings("sd-1", "disk-1")
		Expect(s.storageClassName).To(Equal("iscsi-block"))
		Expect(s.volumeMode).To(Equal(core.PersistentVolumeFilesystem))
		Expect(s.accessMode).To(Equal(core.ReadWriteOnce))
	})

	It("should apply storageClass but keep default volumeMode and accessMode when not specified", func() {
		sm := newStorageMap(api.StoragePair{
			Source:      ref.Ref{ID: "sd-1"},
			Destination: api.DestinationStorage{StorageClass: "iscsi-block"},
		})
		b := newBuilder(&ovirt.Workload{}, sm)

		s := b.resolveLunStorageSettings("sd-1", "disk-1")
		Expect(s.storageClassName).To(Equal("iscsi-block"))
		Expect(s.volumeMode).To(Equal(core.PersistentVolumeBlock))
		Expect(s.accessMode).To(Equal(core.ReadWriteMany))
	})

	It("should apply only volumeMode from map and keep default accessMode", func() {
		sm := newStorageMap(api.StoragePair{
			Source:      ref.Ref{ID: "sd-1"},
			Destination: api.DestinationStorage{StorageClass: "sc", VolumeMode: core.PersistentVolumeFilesystem},
		})
		b := newBuilder(&ovirt.Workload{}, sm)

		s := b.resolveLunStorageSettings("sd-1", "disk-1")
		Expect(s.storageClassName).To(Equal("sc"))
		Expect(s.volumeMode).To(Equal(core.PersistentVolumeFilesystem))
		Expect(s.accessMode).To(Equal(core.ReadWriteMany))
	})

	It("should apply only accessMode from map and keep default volumeMode", func() {
		sm := newStorageMap(api.StoragePair{
			Source:      ref.Ref{ID: "sd-1"},
			Destination: api.DestinationStorage{StorageClass: "sc", AccessMode: core.ReadWriteOnce},
		})
		b := newBuilder(&ovirt.Workload{}, sm)

		s := b.resolveLunStorageSettings("sd-1", "disk-1")
		Expect(s.storageClassName).To(Equal("sc"))
		Expect(s.volumeMode).To(Equal(core.PersistentVolumeBlock))
		Expect(s.accessMode).To(Equal(core.ReadWriteOnce))
	})

	It("should fall back to diskID when storageDomainID is empty", func() {
		sm := newStorageMap(api.StoragePair{
			Source:      ref.Ref{ID: "disk-1"},
			Destination: api.DestinationStorage{StorageClass: "fast-storage", VolumeMode: core.PersistentVolumeBlock, AccessMode: core.ReadWriteOnce},
		})
		b := newBuilder(&ovirt.Workload{}, sm)

		s := b.resolveLunStorageSettings("", "disk-1")
		Expect(s.storageClassName).To(Equal("fast-storage"))
		Expect(s.volumeMode).To(Equal(core.PersistentVolumeBlock))
		Expect(s.accessMode).To(Equal(core.ReadWriteOnce))
	})

	It("should prefer storageDomainID over diskID when both are present", func() {
		sm := newStorageMap(
			api.StoragePair{
				Source:      ref.Ref{ID: "sd-1"},
				Destination: api.DestinationStorage{StorageClass: "from-sd"},
			},
			api.StoragePair{
				Source:      ref.Ref{ID: "disk-1"},
				Destination: api.DestinationStorage{StorageClass: "from-disk"},
			},
		)
		b := newBuilder(&ovirt.Workload{}, sm)

		s := b.resolveLunStorageSettings("sd-1", "disk-1")
		Expect(s.storageClassName).To(Equal("from-sd"))
	})

	It("should return defaults when storageDomainID is empty and diskID is not in the map", func() {
		sm := newStorageMap(api.StoragePair{
			Source:      ref.Ref{ID: "other-disk"},
			Destination: api.DestinationStorage{StorageClass: "fast-storage"},
		})
		b := newBuilder(&ovirt.Workload{}, sm)

		s := b.resolveLunStorageSettings("", "disk-not-mapped")
		Expect(s.storageClassName).To(Equal(""))
		Expect(s.volumeMode).To(Equal(core.PersistentVolumeBlock))
		Expect(s.accessMode).To(Equal(core.ReadWriteMany))
	})
})

// Verify that the plan helper used by newBuilder works.
var _ = Describe("test helpers", func() {
	It("should create a valid builder with storage map", func() {
		vm := &ovirt.Workload{}
		sm := newStorageMap()
		b := newBuilder(vm, sm)
		Expect(b.Context).ToNot(BeNil())
		Expect(b.Context.Map.Storage).To(Equal(sm))
		Expect(b.Context.Plan).ToNot(BeNil())
		Expect(b.Context.Migration).ToNot(BeNil())
	})
})
