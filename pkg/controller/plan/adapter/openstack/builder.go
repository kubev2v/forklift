package openstack

import (
	"fmt"

	"github.com/gophercloud/gophercloud/openstack/blockstorage/extensions/volumeactions"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/v3/snapshots"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/v3/volumes"
	"github.com/gophercloud/gophercloud/openstack/imageservice/v2/images"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/plan"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/ref"
	"github.com/konveyor/forklift-controller/pkg/controller/plan/adapter/base"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/openstack"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/web/openstack"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	libitr "github.com/konveyor/forklift-controller/pkg/lib/itinerary"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	cnv "kubevirt.io/client-go/api/v1"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

// Openstack builder.
type Builder struct {
	*plancontext.Context
}

// Annotations
const (
	AnnImportDiskId = "cdi.kubevirt.io/storage.import.volumeId"
)

// Create the destination Kubevirt VM.
func (r *Builder) VirtualMachine(vmRef ref.Ref, object *cnv.VirtualMachineSpec, persistentVolumeClaims []core.PersistentVolumeClaim) (err error) {
	vm := &model.Workload{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(
			err,
			"VM lookup failed.",
			"vm",
			vmRef.String())
		return
	}

	if object.Template == nil {
		object.Template = &cnv.VirtualMachineInstanceTemplateSpec{}
	}
	r.mapDisks(vm, persistentVolumeClaims, object)
	r.mapCPU(vm, object)
	r.mapMemory(vm, object)
	r.mapFirmware(vm, object)
	r.mapNetworks(vm, object)
	if err != nil {
		return
	}

	return
}

func (r *Builder) mapDisks(vm *model.Workload, persistentVolumeClaims []core.PersistentVolumeClaim, object *cnv.VirtualMachineSpec) {
	var kVolumes []cnv.Volume
	var kDisks []cnv.Disk

	pvcMap := make(map[string]*core.PersistentVolumeClaim)

	for i := range persistentVolumeClaims {
		pvc := &persistentVolumeClaims[i]
		pvcMap[pvc.Annotations[AnnImportDiskId]] = pvc
	}
	for i, av := range vm.AttachedVolumes {
		image := &model.Image{}
		err := r.Source.Inventory.Find(image, ref.Ref{Name: fmt.Sprintf("%s-%s", r.Migration.Name, av.ID)})
		if err != nil {
			return
		}
		pvc := pvcMap[av.ID]
		volumeName := fmt.Sprintf("vol-%v", i)
		volume := cnv.Volume{
			Name: volumeName,
			VolumeSource: cnv.VolumeSource{
				PersistentVolumeClaim: &core.PersistentVolumeClaimVolumeSource{
					ClaimName: pvc.Name,
				},
			},
		}
		disk := cnv.Disk{
			Name: volumeName,
			DiskDevice: cnv.DiskDevice{
				Disk: &cnv.DiskTarget{
					// TODO find where to get this info
					Bus: "virtio",
				},
			},
		}
		kVolumes = append(kVolumes, volume)
		kDisks = append(kDisks, disk)
	}
	object.Template.Spec.Volumes = kVolumes
	object.Template.Spec.Domain.Devices.Disks = kDisks
}
func (r *Builder) mapCPU(vm *model.Workload, object *cnv.VirtualMachineSpec) {
	object.Template.Spec.Domain.Machine = &cnv.Machine{Type: "q35"}
	object.Template.Spec.Domain.CPU = &cnv.CPU{
		Sockets: uint32(1),
		Cores:   uint32(1),
		Threads: uint32(1),
	}
}

func (r *Builder) mapMemory(vm *model.Workload, object *cnv.VirtualMachineSpec) (err error) {
	flavor := &model.Flavor{}
	err = r.Source.Inventory.Find(flavor, ref.Ref{ID: vm.FlavorID})
	if err != nil {
		err = liberr.Wrap(
			err,
			"Flavor lookup failed.",
			"flavor",
			vm.FlavorID)
		return
	}

	reservation := resource.NewQuantity(int64(flavor.RAM*1024*1024), resource.BinarySI)
	object.Template.Spec.Domain.Resources = cnv.ResourceRequirements{
		Requests: map[core.ResourceName]resource.Quantity{
			core.ResourceMemory: *reservation,
		},
	}

	return nil
}

func (r *Builder) mapFirmware(vm *model.Workload, object *cnv.VirtualMachineSpec) {
	features := &cnv.Features{}
	firmware := &cnv.Firmware{}
	firmware.Bootloader = &cnv.Bootloader{BIOS: &cnv.BIOS{}}
	object.Template.Spec.Domain.Features = features
	object.Template.Spec.Domain.Firmware = firmware
}

func (r *Builder) mapNetworks(vm *model.Workload, object *cnv.VirtualMachineSpec) (err error) {
	var kNetworks []cnv.Network
	var kInterfaces []cnv.Interface

	kNetwork := cnv.Network{
		Name:          "netos",
		NetworkSource: cnv.DefaultPodNetwork().NetworkSource,
	}
	kInterface := cnv.Interface{
		Name: "netos",
	}
	kInterface.Masquerade = &cnv.InterfaceMasquerade{}

	kNetworks = append(kNetworks, kNetwork)
	kInterfaces = append(kInterfaces, kInterface)
	object.Template.Spec.Networks = kNetworks
	object.Template.Spec.Domain.Devices.Interfaces = kInterfaces

	return nil
}

// Build tasks.
func (r *Builder) Tasks(vmRef ref.Ref) (list []*plan.Task, err error) {
	vm := &model.Workload{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(
			err,
			"VM lookup failed.",
			"vm",
			vmRef.String())
	}

	for _, va := range vm.AttachedVolumes {
		gb := int64(va.Size)
		list = append(
			list,
			&plan.Task{
				Name: fmt.Sprintf("%s-%s", r.Migration.Name, va.ID),
				Progress: libitr.Progress{
					Total: gb * 1024,
				},
				Annotations: map[string]string{
					"unit": "MB",
				},
			})
	}

	return
}

// Create DataVolume certificate configmap.
func (r *Builder) ConfigMap(_ ref.Ref, in *core.Secret, object *core.ConfigMap) (err error) {
	return
}

func (r *Builder) DataVolumes(vmRef ref.Ref, secret *core.Secret, configMap *core.ConfigMap, dvTemplate *cdi.DataVolume) (dvs []cdi.DataVolume, err error) {
	return nil, nil
}

// Build tasks.
func (r *Builder) TemplateLabels(vmRef ref.Ref) (labels map[string]string, err error) {
	return nil, nil
}

// Return a stable identifier for a DataVolume.
func (r *Builder) ResolveDataVolumeIdentifier(dv *cdi.DataVolume) string {
	return ""
}

// Return a stable identifier for a PersistentDataVolume
func (r *Builder) ResolvePersistentVolumeClaimIdentifier(pvc *core.PersistentVolumeClaim) string {
	return ""
}

// Build credential secret.
func (r *Builder) Secret(_ ref.Ref, in, secret *core.Secret) (err error) {
	secret.StringData = map[string]string{
		"username":    string(in.Data["username"]),
		"password":    string(in.Data["password"]),
		"domainName":  string(in.Data["domainName"]),
		"projectName": string(in.Data["projectName"]),
		"region":      string(in.Data["region"]),
		"insecure":    string(in.Data["insecure"]),
	}
	return
}

func (r *Builder) PodEnvironment(_ ref.Ref, _ *core.Secret) (env []core.EnvVar, err error) {
	return
}

func (r *Builder) PersistentVolumeClaimWithSourceRef(da interface{}, storageName *string, populatorName string, accessModes []core.PersistentVolumeAccessMode, volumeMode *core.PersistentVolumeMode) *core.PersistentVolumeClaim {
	image := da.(*openstack.Image)
	apiGroup := "forklift.konveyor.io"
	size := image.VirtualSize
	if *volumeMode == core.PersistentVolumeFilesystem {
		size = int64(float64(image.VirtualSize) * 1.1)
	}
	return &core.PersistentVolumeClaim{
		ObjectMeta: meta.ObjectMeta{
			Name:      image.ID,
			Namespace: r.Plan.Spec.TargetNamespace,
			Annotations: map[string]string{
				AnnImportDiskId: image.Name[len(r.Migration.Name)+1:],
			},
			Labels: map[string]string{"migration": r.Migration.Name},
		},
		Spec: core.PersistentVolumeClaimSpec{
			AccessModes: accessModes,
			Resources: core.ResourceRequirements{
				Requests: map[core.ResourceName]resource.Quantity{
					core.ResourceStorage: *resource.NewQuantity(size, resource.BinarySI)},
			},
			StorageClassName: storageName,
			VolumeMode:       volumeMode,
			DataSourceRef: &core.TypedLocalObjectReference{
				APIGroup: &apiGroup,
				Kind:     v1beta1.OpenstackVolumePopulatorKind,
				Name:     populatorName,
			},
		},
	}
}

func (r *Builder) BeforeTransferHook(c base.Client, vmRef ref.Ref) (ready bool, err error) {
	// TODO:
	// 1. Dedup
	// 2. Improve concurrency, as soon as the image is ready we can create the PVC, no need to wait
	// for everything to finish
	client, ok := c.(*Client)
	osClient := client.OpenstackClient
	if !ok {
		return false, nil
	}

	vm := &model.Workload{}
	err = r.Source.Inventory.Find(vm, vmRef)
	r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(
			err,
			"VM lookup failed.",
			"vm",
			vmRef.String())
		return true, err
	}

	var snaplist []snapshots.Snapshot
	for _, av := range vm.AttachedVolumes {
		imageName := fmt.Sprintf("%s-%s", r.Migration.Name, av.ID)
		pager := snapshots.List(osClient.BlockStorageService, snapshots.ListOpts{
			Name:  imageName,
			Limit: 1,
		})
		pages, err := pager.AllPages()
		if err != nil {
			return true, err
		}
		isEmpty, err := pages.IsEmpty()
		if err != nil {
			return true, err
		}
		if !isEmpty {
			snaps, err := snapshots.ExtractSnapshots(pages)
			if err != nil {
				return true, err
			}

			snaplist = append(snaplist, snaps...)
			continue
		}

		snapshot, err := snapshots.Create(osClient.BlockStorageService, snapshots.CreateOpts{
			Name:        imageName,
			VolumeID:    av.ID,
			Force:       true,
			Description: imageName,
		}).Extract()
		if err != nil {
			err = liberr.Wrap(
				err,
				"Failed to create snapshot.",
				"volume",
				av.ID)
			return true, err
		}

		snaplist = append(snaplist, *snapshot)
	}

	for _, snap := range snaplist {
		snapshot, err := snapshots.Get(osClient.BlockStorageService, snap.ID).Extract()
		if err != nil {
			return true, err
		}
		if snapshot.Status != "available" {
			r.Log.Info("Snapshot not ready yet, recheking...", "snapshot", snap.Name)
			return false, nil
		}
	}

	var vollist []volumes.Volume
	for _, snap := range snaplist {
		imageName := fmt.Sprintf("%s-%s", r.Migration.Name, snap.VolumeID)
		pager := volumes.List(osClient.BlockStorageService, volumes.ListOpts{
			Name:  imageName,
			Limit: 1,
		})
		pages, err := pager.AllPages()
		if err != nil {
			return true, err
		}
		isEmpty, err := pages.IsEmpty()
		if err != nil {
			return true, err
		}
		if !isEmpty {
			vols, err := volumes.ExtractVolumes(pages)
			if err != nil {
				return true, err
			}
			vollist = append(vollist, vols...)

			continue
		}
		volume, err := volumes.Create(osClient.BlockStorageService, volumes.CreateOpts{
			Name:        imageName,
			SnapshotID:  snap.ID,
			Size:        snap.Size,
			Description: imageName,
		}).Extract()
		if err != nil {
			err = liberr.Wrap(
				err,
				"Failed to create snapshot.",
				"volume",
				volume.ID)
			return true, err
		}
		vollist = append(vollist, *volume)
	}

	for _, vol := range vollist {
		volume, err := volumes.Get(osClient.BlockStorageService, vol.ID).Extract()
		if err != nil {
			return true, err
		}

		if volume.Status != "available" && volume.Status != "uploading" {
			r.Log.Info("Volume not ready yet, recheking...", "volume", vol.Name)
			return false, nil
		}
	}

	var imagelist []string

	for _, vol := range vollist {
		pager := images.List(osClient.ImageService, images.ListOpts{
			Name:  vol.Description,
			Limit: 1,
		})
		pages, err := pager.AllPages()
		if err != nil {
			return true, err
		}
		isEmpty, err := pages.IsEmpty()
		if err != nil {
			return true, err
		}
		if !isEmpty {
			imgs, err := images.ExtractImages(pages)
			if err != nil {
				return true, err
			}
			for _, i := range imgs {
				imagelist = append(imagelist, i.ID)
			}
			r.Log.Info("Image already exists", "id", imagelist)
			continue
		}

		image, err := volumeactions.UploadImage(osClient.BlockStorageService, vol.ID, volumeactions.UploadImageOpts{
			ImageName:  vol.Description,
			DiskFormat: "raw",
		}).Extract()
		if err != nil {
			err = liberr.Wrap(
				err,
				"Failed to create image.",
				"image",
				image.ImageID)
			return false, err
		}

		imagelist = append(imagelist, image.ImageID)
	}

	for _, imageID := range imagelist {
		img, err := images.Get(osClient.ImageService, imageID).Extract()
		if err != nil {
			return true, err
		}

		// TODO also check for "saving" and "error"
		if img.Status != images.ImageStatusActive {
			r.Log.Info("Image not ready yet, recheking...", "image", img)
			return false, nil
		} else if img.Status == images.ImageStatusActive {
			// TODO figure out a better way, since when the image in the inventory may be out of sync
			// with openstack, and be ready in openstack, but not in the inventory
			if !r.imageReady(img.Name) {
				r.Log.Info("Image not ready yet in inventory, recheking...", "image", img.Name)
				return false, nil
			} else {
				r.Log.Info("Image is ready, cleaning up...", "image", img.Name)
				r.cleanup(c, img.Name)
			}
		}
	}

	return true, nil
}

func (r *Builder) imageReady(imageName string) bool {
	image := &model.Image{}
	err := r.Source.Inventory.Find(image, ref.Ref{Name: imageName})
	if err == nil {
		r.Log.Info("Image status in inventory", "image", image.Status)
		return image.Status == "active"
	}
	return false
}

func (r *Builder) cleanup(c base.Client, imageName string) {
	client, ok := c.(*Client)
	osClient := client.OpenstackClient
	if !ok {
		r.Log.Info("Couldn't cast client (should never happen)")
		return
	}

	volume := &model.Volume{}
	err := r.Source.Inventory.Find(volume, ref.Ref{Name: imageName})
	if err != nil {
		r.Log.Error(err, "Couldn't find volume", "name", imageName)
	}

	volumes.Delete(osClient.BlockStorageService, volume.ID, volumes.DeleteOpts{Cascade: true})

	snapshot := &model.Snapshot{}
	err = r.Source.Inventory.Find(snapshot, ref.Ref{Name: imageName})
	if err != nil {
		r.Log.Error(err, "Couldn't find snapshot", "name", imageName)
	}

	snapshots.Delete(osClient.BlockStorageService, snapshot.ID)
}
