package openstack

import (
	"errors"
	"strings"
	"time"

	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/controller/plan/util"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/openstack"
	libclient "github.com/kubev2v/forklift/pkg/lib/client/openstack"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/settings"
	"k8s.io/apimachinery/pkg/util/wait"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

const (
	ImageStatusActive    = libclient.ImageStatusActive
	ImageStatusImporting = libclient.ImageStatusImporting
	ImageStatusQueued    = libclient.ImageStatusQueued
	ImageStatusSaving    = libclient.ImageStatusSaving
	ImageStatusUploading = libclient.ImageStatusUploading

	SnapshotStatusAvailable = libclient.SnapshotStatusAvailable
	SnapshotStatusCreating  = libclient.SnapshotStatusCreating
	SnapshotStatusDeleting  = libclient.SnapshotStatusDeleting
	SnapshotStatusDeleted   = libclient.SnapshotStatusDeleted

	VolumeStatusAvailable = libclient.VolumeStatusAvailable
	VolumeStatusInUse     = libclient.VolumeStatusInUse
	VolumeStatusCreating  = libclient.VolumeStatusCreating
	VolumeStatusDeleting  = libclient.VolumeStatusDeleting
	VolumeStatusUploading = libclient.VolumeStatusUploading
)

var ResourceNotFoundError = errors.New("resource not found")
var NameOrIDRequiredError = errors.New("id or name is required")
var UnexpectedVolumeStatusError = errors.New("unexpected volume status")

type Client struct {
	libclient.Client
	Context *plancontext.Context
}

// Connect.
func (r *Client) connect() (err error) {
	r.URL = r.Context.Source.Provider.Spec.URL
	r.LoadOptionsFromSecret(r.Context.Source.Secret)
	err = r.Connect()
	return
}

// Power on the source VM.
func (r *Client) PowerOn(vmRef ref.Ref) (err error) {
	err = r.VMStart(vmRef.ID)
	if err != nil {
		err = liberr.Wrap(err)
	}
	return
}

// Power off the source VM.
func (r *Client) PowerOff(vmRef ref.Ref) (err error) {
	poweredOff, err := r.PoweredOff(vmRef)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	if !poweredOff {
		err = r.VMStop(vmRef.ID)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
	}
	return
}

// Return the source VM's power state.
func (r *Client) PowerState(vmRef ref.Ref) (state planapi.VMPowerState, err error) {
	status, err := r.VMStatus(vmRef.ID)
	if err != nil {
		err = liberr.Wrap(err)
		state = planapi.VMPowerStateUnknown
		return
	}
	switch status {
	case libclient.VmStatusActive:
		state = planapi.VMPowerStateOn
	case libclient.VmStatusShutoff:
		state = planapi.VMPowerStateOff
	default:
		state = planapi.VMPowerStateUnknown
	}
	return
}

// Return whether the source VM is powered off.
func (r *Client) PoweredOff(vmRef ref.Ref) (off bool, err error) {
	state, err := r.PowerState(vmRef)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	off = state == planapi.VMPowerStateOff
	return
}

// Create a snapshot of the source VM.
func (r *Client) CreateSnapshot(vmRef ref.Ref, hostsFunc util.HostsFunc) (snapshotId string, creationTaskId string, err error) {
	return
}

// Check if a snapshot is ready to transfer.
func (r *Client) CheckSnapshotReady(vmRef ref.Ref, precopy planapi.Precopy, hosts util.HostsFunc) (ready bool, snapshotId string, err error) {
	return
}

// CheckSnapshotRemove implements base.Client
func (r *Client) CheckSnapshotRemove(vmRef ref.Ref, precopy planapi.Precopy, hosts util.HostsFunc) (bool, error) {
	return false, nil
}

// Set DataVolume checkpoints.
func (r *Client) SetCheckpoints(vmRef ref.Ref, precopies []planapi.Precopy, datavolumes []cdi.DataVolume, final bool, hostsFunc util.HostsFunc) error {
	return nil
}

// Remove a VM snapshot. No-op for this provider.
func (r *Client) RemoveSnapshot(vmRef ref.Ref, snapshot string, hostsFunc util.HostsFunc) (removeTaskId string, err error) {
	return
}

// Get disk deltas for a VM snapshot. No-op for this provider.
func (r *Client) GetSnapshotDeltas(vmRef ref.Ref, snapshot string, hostsFunc util.HostsFunc) (s map[string]string, err error) {
	return
}

// Close connections to the provider API.
func (r *Client) Close() {
}

func (r *Client) Finalize(vmStatuses []*planapi.VMStatus, migrationName string) {
	for _, vmStatus := range vmStatuses {
		vmRef := ref.Ref{ID: vmStatus.Ref.ID}
		vm, err := r.getVM(vmRef)
		if err != nil {
			r.Log.Error(err, "failed to find vm", "vm", vm.Name)
			return
		}
		err = r.removeImagesFromVolumes(vm)
		if err != nil {
			r.Log.Error(err, "removing the images from volumes", "vm", vm.Name)
			return
		}
		err = r.removeVmSnapshotImage(vm)
		if err != nil {
			r.Log.Error(err, "removing the vm snapshot image", "vm", vm.Name)
			return
		}
	}
}

func (r *Client) removeImagesFromVolumes(vm *libclient.VM) (err error) {
	images, err := r.getImagesFromVolumes(vm)
	if err != nil {
		r.Log.Error(err, "failed to retrieve the list of images",
			"vm", vm.Name)
		return
	}
	for _, image := range images {
		switch image.Status {
		case ImageStatusActive:
			err = r.Delete(&image)
			if err != nil {
				r.Log.Error(err, "trying to remove an image",
					"vm", vm.Name, "image", image.Name)
				return
			}
		default:
			r.Log.Info("unexpected image status when finalizing, the image will remain",
				"vm", vm.Name, "image", image.Name)
		}
	}
	return
}

func (r *Client) DetachDisks(vmRef ref.Ref) (err error) {
	// no-op
	return
}

func (r *Client) PreTransferActions(vmRef ref.Ref) (ready bool, err error) {
	vm, err := r.getVM(vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}
	ready, err = r.ensureVmSnapshot(vm)
	if err != nil || !ready {
		return
	}

	ready, err = r.ensureImagesFromVolumesReady(vm)
	if err != nil || ready {
		return
	}

	err = r.ensureSnapshotsFromVolumes(vm)
	if err != nil {
		return
	}

	err = r.ensureVolumesFromSnapshots(vm)
	return
}

func (r *Client) getVM(vmRef ref.Ref) (vm *libclient.VM, err error) {
	if vmRef.ID == "" && vmRef.Name == "" {
		err = NameOrIDRequiredError
		return
	}
	if vmRef.ID != "" {
		vm = &libclient.VM{}
		err = r.Get(vm, vmRef.ID)
		if err != nil {
			if r.IsNotFound(err) {
				err = ResourceNotFoundError
			}
			return
		}
	}
	if vmRef.Name != "" {
		vms := []libclient.VM{}
		opts := libclient.VMListOpts{}
		opts.Name = vmRef.Name
		opts.Limit = 1
		err = r.List(&vms, &opts)
		if err != nil {
			return
		}
		if len(vms) == 0 {
			err = ResourceNotFoundError
			return
		}
		vm = &vms[0]
	}
	return
}

// Get the Image by ref.
func (r *Client) getImage(imageRef ref.Ref) (image *libclient.Image, err error) {
	if imageRef.ID == "" && imageRef.Name == "" {
		err = NameOrIDRequiredError
		return
	}
	if imageRef.ID != "" {
		image = &libclient.Image{}
		err = r.Get(image, imageRef.ID)
		if err != nil {
			if r.IsNotFound(err) {
				err = ResourceNotFoundError
			}
			return
		}
	}
	if imageRef.Name != "" {
		images := []libclient.Image{}
		opts := libclient.ImageListOpts{}
		opts.Name = imageRef.Name
		opts.SortKey = "created_at"
		opts.SortDir = "desc"
		opts.Limit = 1
		err = r.List(&images, &opts)
		if err != nil {
			return
		}
		if len(images) == 0 {
			err = ResourceNotFoundError
			return
		}
		image = &images[0]
	}

	return
}

// Get the Volume by ref.
func (r *Client) getVolume(volumeRef ref.Ref) (volume *libclient.Volume, err error) {
	if volumeRef.ID == "" && volumeRef.Name == "" {
		err = NameOrIDRequiredError
		return
	}
	if volumeRef.ID != "" {
		volume = &libclient.Volume{}
		err = r.Get(volume, volumeRef.ID)
		if err != nil {
			if err != nil {
				if r.IsNotFound(err) {
					err = ResourceNotFoundError
				}
				return
			}
		}
	}
	if volumeRef.Name != "" {
		volumes := []libclient.Volume{}
		opts := libclient.VolumeListOpts{}
		opts.Name = volumeRef.Name
		opts.Sort = "created_at:desc"
		opts.Limit = 1
		err = r.List(&volumes, &opts)
		if err != nil {
			return
		}
		if len(volumes) == 0 {
			err = ResourceNotFoundError
			return
		}
		volume = &volumes[0]
	}

	return
}

func (r *Client) cleanup(vm *libclient.VM, originalVolumeID string) (err error) {
	r.Log.Info("cleaning up the snapshot and the volume created from it",
		"vm", vm.Name, "originalVolumeID", originalVolumeID)
	snapshot, err := r.getSnapshotFromVolume(vm, originalVolumeID)
	if err != nil {
		if !errors.Is(err, ResourceNotFoundError) {
			err = liberr.Wrap(err)
			r.Log.Error(err, "retrieving snapshot from volume information when cleaning up",
				"vm", vm.Name, "volumeID", originalVolumeID)
			err = nil
			return
		}
		r.Log.Info("the snapshot from volume cannot be found, skipping clean up...",
			"vm", vm.Name, "volumeID", originalVolumeID)
		err = nil
		return
	}
	r.Log.Info("cleaning up the volume from snapshot",
		"vm", vm.Name, "snapshotID", snapshot.ID)
	err = r.removeVolumeFromSnapshot(vm, snapshot.ID)
	if err != nil {
		err = liberr.Wrap(err)
		r.Log.Error(err, "removing volume from snapshot when cleaning up",
			"vmID", vm.ID, "snapshotID", snapshot.ID)
		err = nil
		return
	}

	// Now we need to wait for the volume to be removed
	condition := func() (done bool, err error) {
		volume, err := r.getVolumeFromSnapshot(vm, snapshot.ID)
		if err != nil {
			if errors.Is(err, ResourceNotFoundError) {
				r.Log.Info("volume doesn't exist, assuming we are done")
				done = true
				err = nil
				return
			}
			err = liberr.Wrap(err)
			r.Log.Error(err, "retrieving volume from snapshot information when cleaning up",
				"vm", vm.Name, "snapshotID", snapshot.ID)
			return
		}
		switch volume.Status {
		case VolumeStatusDeleting:
			r.Log.Info("the volume is still being deleted, waiting...",
				"vm", vm.Name, "volume", volume.Name, "snapshot", volume.SnapshotID)
		default:
			err = UnexpectedVolumeStatusError
			r.Log.Error(err, "checking the volume",
				"vm", vm.Name, "volume", volume.Name, "status", volume.Status)
			return
		}
		return
	}

	backoff := wait.Backoff{
		Duration: 3 * time.Second,
		Factor:   1.5,
		Steps:    settings.Settings.CleanupRetries,
	}

	err = wait.ExponentialBackoff(backoff, condition)
	if err != nil {
		err = liberr.Wrap(err)
		r.Log.Error(err, "waiting for the volume to be removed",
			"vm", vm.Name, "snapshotID", snapshot.ID)
		return
	}

	r.Log.Info("cleaning up the snapshot from volume",
		"vm", vm.Name, "originalVolumeID", originalVolumeID)
	err = r.removeSnapshotFromVolume(vm, originalVolumeID)
	if err != nil {
		err = liberr.Wrap(err)
		r.Log.Error(err, "removing snapshot from volume when cleaning up",
			"vmID", vm.ID, "volumeID", originalVolumeID)
		err = nil
	}
	return
}

func (r *Client) updateImageProperty(vm *libclient.VM, image *libclient.Image) (err error) {
	volumesFromSnapshots, err := r.getVolumesFromSnapshots(vm)
	found := false
	for _, volumeFromSnapshot := range volumesFromSnapshots {
		originalVolumeID := volumeFromSnapshot.Metadata[forkliftPropertyOriginalVolumeID]
		imageFromVolumeName := getImageFromVolumeName(r.Context, vm.ID, originalVolumeID)
		if image.Name == imageFromVolumeName {
			found = true
			imageUpdateOpts := &libclient.ImageUpdateOpts{}
			imageUpdateOpts.AddImageProperty(forkliftPropertyOriginalVolumeID, originalVolumeID)
			err = r.Update(image, imageUpdateOpts)
		}
	}
	if !found {
		r.Log.Info("cannot find the original volume id within the metadata", "vm", vm.Name, "image", image.Name)
	}
	return
}

func (r *Client) createSnapshotFromVolume(vm *libclient.VM, volumeID string) (snapshot *libclient.Snapshot, err error) {
	snapshotName := getSnapshotFromVolumeName(r.Context, vm.ID)
	opts := &libclient.SnapshotCreateOpts{}
	opts.Name = snapshotName
	opts.VolumeID = volumeID
	opts.Force = true
	opts.Metadata = map[string]string{
		forkliftPropertyOriginalVolumeID: volumeID,
	}
	snapshot = &libclient.Snapshot{}
	err = r.Create(snapshot, opts)
	if err != nil {
		err = liberr.Wrap(err)
	}
	return
}

func (r *Client) createVolumeFromSnapshot(vm *libclient.VM, snapshotID string) (volume *libclient.Volume, err error) {
	snapshot := &libclient.Snapshot{}
	err = r.Get(snapshot, snapshotID)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	opts := &libclient.VolumeCreateOpts{}
	metadata := map[string]string{
		forkliftPropertyOriginalVolumeID: snapshot.VolumeID,
	}
	volumeName := getVolumeFromSnapshotName(r.Context, vm.ID, snapshot.ID)
	opts.Name = volumeName
	opts.SnapshotID = snapshotID
	opts.Metadata = metadata
	volume = &libclient.Volume{}
	err = r.Create(volume, opts)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	return
}

func (r *Client) createImageFromVolume(vm *libclient.VM, volumeID string) (image *libclient.Image, err error) {
	volumeRef := ref.Ref{ID: volumeID}
	volume, err := r.getVolume(volumeRef)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	// Workaround for https://bugs.launchpad.net/cinder/+bug/1945500
	for key := range volume.VolumeImageMetadata {
		if strings.HasPrefix(key, "os_glance") {
			err = r.UnsetImageMetadata(volumeID, key)
			if err != nil {
				err = liberr.Wrap(err, "vm", vm.Name, "volumeID", volumeID, "key", key)
				return
			}
		}
	}
	// end Workaround
	imageName := getImageFromVolumeName(r.Context, vm.ID, volume.Metadata[forkliftPropertyOriginalVolumeID])
	image, err = r.UploadImage(imageName, volume.ID)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	return
}

// Create a image of the source VM.
func (r *Client) createVmSnapshotImage(vm *libclient.VM) (vmImage *libclient.Image, err error) {
	vmSnapshotImageName := getVmSnapshotName(r.Context, vm.ID)
	opts := &libclient.VMCreateImageOpts{}
	opts.Name = vmSnapshotImageName
	vmImage, err = r.VMCreateSnapshotImage(vm.ID, *opts)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	// The vm is image based and we need to create the snapshots of the
	// volumes attached to it.
	if imageID, ok := vm.Image["id"]; ok {
		// Update property for image based
		imageUpdateOpts := &libclient.ImageUpdateOpts{}
		imageUpdateOpts.AddImageProperty(forkliftPropertyOriginalImageID, imageID.(string))
		err = r.Update(vmImage, imageUpdateOpts)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		for _, attachedVolume := range vm.AttachedVolumes {
			var volume *libclient.Volume
			volume, err = r.getVolume(ref.Ref{ID: attachedVolume.ID})
			if err != nil {
				err = liberr.Wrap(err)
				return
			}
			switch volume.Status {
			case VolumeStatusInUse:
				_, err = r.createSnapshotFromVolume(vm, attachedVolume.ID)
				if err != nil {
					err = liberr.Wrap(err)
					return
				}
			default:
				err = UnexpectedVolumeStatusError
				r.Log.Error(err, "creating snapshots from volumes",
					"vm", vm.Name, "volume", volume.Name)
				return
			}
		}
	}

	return
}

func (r *Client) getSnapshotFromVolume(vm *libclient.VM, volumeID string) (snapshot *libclient.Snapshot, err error) {
	snapshotName := getSnapshotFromVolumeName(r.Context, vm.ID)
	snapshots := []libclient.Snapshot{}
	opts := libclient.SnapshotListOpts{}
	opts.Name = snapshotName
	opts.VolumeID = volumeID
	opts.Limit = 1
	err = r.List(&snapshots, &opts)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	if len(snapshots) == 0 {
		err = ResourceNotFoundError
		return
	}
	snapshot = &snapshots[0]
	return
}

func (r *Client) getVolumeFromSnapshot(vm *libclient.VM, snapshotID string) (volume *libclient.Volume, err error) {
	snapshot := &libclient.Snapshot{}
	err = r.Get(snapshot, snapshotID)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	volumeName := getVolumeFromSnapshotName(r.Context, vm.ID, snapshot.ID)
	volumes := []libclient.Volume{}
	metadata := map[string]string{
		forkliftPropertyOriginalVolumeID: snapshot.VolumeID,
	}
	opts := libclient.VolumeListOpts{}
	opts.Name = volumeName
	opts.Metadata = metadata
	opts.Limit = 1
	err = r.List(&volumes, &opts)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	if len(volumes) == 0 {
		err = ResourceNotFoundError
		return
	}
	volume = &volumes[0]
	return
}

func (r *Client) getImageFromVolume(vm *libclient.VM, volumeID string) (image *libclient.Image, err error) {
	volumeRef := ref.Ref{ID: volumeID}
	volume, err := r.getVolume(volumeRef)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	originalVolumeID := volume.Metadata[forkliftPropertyOriginalVolumeID]
	imageName := getImageFromVolumeName(r.Context, vm.ID, originalVolumeID)
	images := []libclient.Image{}
	opts := libclient.ImageListOpts{}
	opts.Name = imageName
	opts.SortKey = "created_at"
	opts.SortDir = "desc"
	opts.Limit = 1
	err = r.List(&images, &opts)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	if len(images) == 0 {
		err = ResourceNotFoundError
		return
	}
	image = &images[0]

	return
}

func (r *Client) getVmSnapshotImage(vm *libclient.VM) (vmImage *libclient.Image, err error) {
	vmSnapshotImageName := getVmSnapshotName(r.Context, vm.ID)
	opts := &libclient.ImageListOpts{}
	opts.Name = vmSnapshotImageName
	opts.Limit = 1
	opts.SortKey = "created_at"
	opts.SortDir = "desc"
	images, err := r.VMGetSnapshotImages(opts)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	if len(images) == 0 {
		err = ResourceNotFoundError
		return
	}
	vmImage = &images[0]
	return
}

func (r *Client) removeSnapshotFromVolume(vm *libclient.VM, volumeID string) (err error) {
	snapshot, err := r.getSnapshotFromVolume(vm, volumeID)
	if err != nil {
		if errors.Is(err, ResourceNotFoundError) {
			err = nil
			return
		}
		err = liberr.Wrap(err)
		return
	}
	switch snapshot.Status {
	case SnapshotStatusAvailable:
		err = r.Delete(snapshot)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
	case SnapshotStatusDeleted, SnapshotStatusDeleting:
		return
	default:
		err = liberr.New("unexpected snapshot status")
		r.Log.Error(err, "removing snapshot from volume",
			"vm", vm.Name, "volumeID", volumeID, "snapshotID", snapshot.ID, "status", snapshot.Status)
		return
	}
	return
}

func (r *Client) removeVolumeFromSnapshot(vm *libclient.VM, snapshotID string) (err error) {
	var volume *libclient.Volume
	volume, err = r.getVolumeFromSnapshot(vm, snapshotID)
	if err != nil {
		if errors.Is(err, ResourceNotFoundError) {
			err = nil
			return
		}
		err = liberr.Wrap(err)
		return
	}
	switch volume.Status {
	case VolumeStatusAvailable:
		err = r.Delete(volume)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
	case VolumeStatusDeleting:
		return
	default:
		err = UnexpectedVolumeStatusError
		r.Log.Error(err, "removing volume from snapshot",
			"vm", vm.Name, "volume", volume.ID, "snapshot", snapshotID, "status", volume.Status)
		return
	}
	return
}

// Remove vm image.
func (r *Client) removeVmSnapshotImage(vm *libclient.VM) (err error) {
	image, err := r.getVmSnapshotImage(vm)
	if err != nil {
		if errors.Is(err, ResourceNotFoundError) {
			err = nil
			return
		}
		err = liberr.Wrap(err)
		return
	}
	switch image.Status {
	case ImageStatusActive:
		err = r.VMRemoveSnapshotImage(image.ID)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
	case libclient.ImageStatusDeleted:
		return
	default:
		err = liberr.New("unexpected image status")
		r.Log.Error(err, "removing image from volume",
			"vm", vm.Name, "image", image.Name, "status", image.Status)
		return
	}
	return
}

// Retrieves the snapshots created from the VM's attached volumes.
func (r *Client) getSnapshotsFromVolumes(vm *libclient.VM) (snapshots []libclient.Snapshot, err error) {
	var volumeSnapshot *libclient.Snapshot
	for _, volume := range vm.AttachedVolumes {
		volumeSnapshot, err = r.getSnapshotFromVolume(vm, volume.ID)
		if err != nil {
			if errors.Is(err, ResourceNotFoundError) {
				r.Log.Info("volume not found", "vmID", vm.ID, "volumeID", volume.ID)
				err = nil
				continue
			}
			err = liberr.Wrap(err)
			return
		}
		snapshots = append(snapshots, *volumeSnapshot)
	}
	return
}

// Retrieves the volumes created from the snapshots.
func (r *Client) getVolumesFromSnapshots(vm *libclient.VM) (volumes []libclient.Volume, err error) {
	var snapshots []libclient.Snapshot
	snapshots, err = r.getSnapshotsFromVolumes(vm)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	for _, snapshot := range snapshots {
		var volume *libclient.Volume
		volume, err = r.getVolumeFromSnapshot(vm, snapshot.ID)
		if err != nil {
			if errors.Is(err, ResourceNotFoundError) {
				r.Log.Info("volume not found", "vmID", vm.ID, "snapshotID", snapshot.ID)
				err = nil
				continue
			}
			err = liberr.Wrap(err)
			return
		}
		volumes = append(volumes, *volume)
	}
	return
}

// Retrieves the images created from the volume snapshots.
func (r *Client) getImagesFromVolumes(vm *libclient.VM) (images []libclient.Image, err error) {
	for _, attachedVolume := range vm.AttachedVolumes {
		imageName := getImageFromVolumeName(r.Context, vm.ID, attachedVolume.ID)
		var image *libclient.Image
		image, err = r.getImage(ref.Ref{Name: imageName})
		if err != nil {
			if errors.Is(err, ResourceNotFoundError) {
				r.Log.Info("image not found", "vmID", vm.ID, "volumeID", attachedVolume.ID)
				err = nil
				continue
			}
			err = liberr.Wrap(err)
			return
		}
		images = append(images, *image)
	}
	return
}

func (r *Client) ensureVmSnapshot(vm *libclient.VM) (ready bool, err error) {
	vmSnapshotImage, err := r.getVmSnapshotImage(vm)
	if err != nil {
		if !errors.Is(err, ResourceNotFoundError) {
			err = liberr.Wrap(err)
			r.Log.Error(err, "trying to retrieve the VM snapshot image info",
				"vm", vm.Name)
			return
		}
		r.Log.Info("creating the VM snapshot image", "vm", vm.Name)
		vmSnapshotImage, err = r.createVmSnapshotImage(vm)
		if err != nil {
			err = liberr.Wrap(err)
			r.Log.Error(err, "trying to create the VM snapshot image",
				"vm", vm.Name)
			return
		}
	}
	switch vmSnapshotImage.Status {
	case ImageStatusActive:
		r.Log.Info("the VM snapshot image is ready!",
			"vm", vm.Name, "image", vmSnapshotImage.Name, "imageID", vmSnapshotImage.ID)
		ready = true
		if _, ok := vm.Image["id"]; ok {
			r.Log.Info("the VM is image based, checking the image properties", "vm", vm.Name, "snapshot", vmSnapshotImage.Name)
			ready, err = r.ensureImageUpToDate(vm, vmSnapshotImage, vmTypeImageBased)
			if err != nil {
				r.Log.Error(err, "checking the VM snapshot image properties", "vm", vm.Name, "image", vmSnapshotImage.Name)
				return
			}

			if !ready {
				r.Log.Info("the VM snapshot image properties are not in sync, skipping...", "vm", vm.Name, "image", vmSnapshotImage.Name)
				return
			}
		}
		return
	case ImageStatusImporting, ImageStatusQueued, ImageStatusUploading, ImageStatusSaving:
		r.Log.Info("the VM snapshot image is not ready yet, skipping...",
			"vm", vm.Name, "image", vmSnapshotImage.Name, "imageID", vmSnapshotImage.ID)
		return
	default:
		err = liberr.New("unexpected VM snapshot image status")
		r.Log.Error(err, "checking the VM snapshot image",
			"vm", vm.Name, "image", vmSnapshotImage.Name, "imageID", vmSnapshotImage.ID, "status", vmSnapshotImage.Status)
		return
	}
}

func (r *Client) ensureImagesFromVolumesReady(vm *libclient.VM) (ready bool, err error) {
	var imagesFromVolumes []libclient.Image
	if imagesFromVolumes, err = r.getImagesFromVolumes(vm); err != nil {
		err = liberr.Wrap(err)
		r.Log.Error(err, "error while trying to get the images from the VM volumes",
			"vm", vm.Name)
		return
	}
	if len(vm.AttachedVolumes) != len(imagesFromVolumes) {
		r.Log.Info("not all the images have been created",
			"vm", vm.Name, "attachedVolumes", vm.AttachedVolumes, "imagesFromVolumes", imagesFromVolumes)
		return
	}
	for _, image := range imagesFromVolumes {
		imageReady, imageReadyErr := r.ensureImageFromVolumeReady(vm, &image)
		switch {
		case imageReadyErr != nil:
			err = liberr.Wrap(imageReadyErr)
			return
		case !imageReady:
			r.Log.Info("found an image that is not ready",
				"vm", vm.Name, "image", image.Name)
			return
		case imageReady:
			originalVolumeID := image.Properties[forkliftPropertyOriginalVolumeID].(string)
			ready, err := r.isImageReadyInInventory(vm, &image)
			if err != nil {
				return false, liberr.Wrap(err)
			}
			if !ready {
				return false, nil
			}
			r.Log.Info("the image is ready in the inventory",
				"vm", vm.Name, "image", image.Name, "properties", image.Properties)

			go func() {
				// executing this in a non-blocking mode
				err := r.cleanup(vm, originalVolumeID)
				if err != nil {
					r.Log.Error(err, "failed to cleanup snapshot and volume",
						"vm", vm.Name, "volumeId", originalVolumeID)
				}
			}()
		}

	}

	ready = true
	r.Log.Info("all steps finished!", "vm", vm.Name)
	return
}

func (r *Client) isImageReadyInInventory(vm *libclient.VM, image *libclient.Image) (ready bool, err error) {
	// Check that the inventory is synchronized with the images
	inventoryImage := &model.Image{}
	err = r.Context.Source.Inventory.Find(inventoryImage, ref.Ref{ID: image.ID})
	if err != nil {
		if errors.As(err, &model.NotFoundError{}) {
			err = nil
			r.Log.Info("the image does not exist in the inventory, waiting...",
				"vm", vm.Name, "image", image.Name, "properties", image.Properties)
			return
		}
		err = liberr.Wrap(err)
		r.Log.Error(err, "trying to find the image in the inventory",
			"vm", vm.Name, "image", image.Name, "properties", image.Properties)
		return
	}

	if inventoryImage.Status != string(ImageStatusActive) {
		r.Log.Info("the image is not ready in the inventory, waiting...",
			"vm", vm.Name, "image", image.Name, "properties", image.Properties)
		return
	}

	return true, nil
}

func (r *Client) ensureImageFromVolumeReady(vm *libclient.VM, image *libclient.Image) (ready bool, err error) {
	switch image.Status {
	case ImageStatusQueued, ImageStatusUploading, ImageStatusSaving:
		r.Log.Info("the image is still being processed",
			"vm", vm.Name, "image", image.Name, "status", image.Status)
	case ImageStatusActive:
		err = r.updateImageProperty(vm, image)
		if err != nil {
			return
		}
		r.Log.Info("the image properties have been updated",
			"vm", vm.Name, "image", image.Name, "properties", image.Properties)
		var imageUpToDate bool
		imageUpToDate, err = r.ensureImageUpToDate(vm, image, vmTypeVolumeBased)
		if err != nil || !imageUpToDate {
			return
		}
		r.Log.Info("the image properties are in sync, cleaning the image",
			"vm", vm.Name, "image", image.Name, "properties", image.Properties)
		ready = true
	default:
		err = liberr.New("unexpected image status")
		r.Log.Error(err, "checking the image from volume",
			"vm", vm.Name, "image", image.Name, "status", image.Status)
	}
	return
}

type vmType string

var vmTypeImageBased vmType = "imageBased"
var vmTypeVolumeBased vmType = "volumeBased"

func (r *Client) ensureImageUpToDate(vm *libclient.VM, image *libclient.Image, vmType vmType) (upToDate bool, err error) {
	inventoryImage := &model.Image{}
	if err = r.Context.Source.Inventory.Find(inventoryImage, ref.Ref{ID: image.ID}); err != nil {
		if errors.As(err, &model.NotFoundError{}) {
			err = nil
			r.Log.Info("the image does not exist in the inventory, waiting...",
				"vm", vm.Name, "image", image.Name, "properties", image.Properties)
		}
		return
	}

	if inventoryImage.Status != string(image.Status) {
		r.Log.Info("image status is not in sync, waiting...",
			"vm", vm.Name, "image", inventoryImage.Name, "status", inventoryImage.Status)
		return
	}

	switch vmType {
	case vmTypeImageBased:
		_, upToDate = inventoryImage.Properties[forkliftPropertyOriginalImageID]
	case vmTypeVolumeBased:
		_, upToDate = inventoryImage.Properties[forkliftPropertyOriginalVolumeID]
	}

	if !upToDate {
		r.Log.Info("image properties have not been synchronized, waiting...",
			"vm", vm.Name, "image", inventoryImage.Name, "properties", inventoryImage.Properties)
	}

	return
}

func (r *Client) ensureSnapshotsFromVolumes(vm *libclient.VM) (err error) {
	var snapshotsFromVolumes []libclient.Snapshot
	if snapshotsFromVolumes, err = r.getSnapshotsFromVolumes(vm); err != nil {
		err = liberr.Wrap(err)
		return
	}
	for _, snapshot := range snapshotsFromVolumes {
		switch snapshot.Status {
		case SnapshotStatusCreating:
			r.Log.Info("the snapshot is still being created, skipping...",
				"vm", vm.Name, "snapshot", snapshot.Name)
		case SnapshotStatusAvailable:
			err = r.ensureVolumeFromSnapshot(vm, &snapshot)
		case SnapshotStatusDeleted, SnapshotStatusDeleting:
			r.Log.Info("the snapshot is being deleted, skipping...",
				"vm", vm.Name, "snapshot", snapshot.Name)
		default:
			err = liberr.New("unexpected snapshot status")
			r.Log.Error(err, "checking the snapshot",
				"vm", vm.Name, "snapshot", snapshot.Name, "status", snapshot.Status)
			return
		}
	}
	return
}

func (r *Client) ensureVolumeFromSnapshot(vm *libclient.VM, snapshot *libclient.Snapshot) (err error) {
	if _, err = r.getVolumeFromSnapshot(vm, snapshot.ID); err != nil {
		if !errors.Is(err, ResourceNotFoundError) {
			err = liberr.Wrap(err)
			r.Log.Error(err, "trying to get the snapshot info from the volume  VM snapshot",
				"vm", vm.Name, "snapshot", snapshot.Name)
			return
		}
		imageName := getImageFromVolumeName(r.Context, vm.ID, snapshot.VolumeID)
		var image *libclient.Image
		image, err = r.getImage(ref.Ref{Name: imageName})
		if err == nil {
			r.Log.Info("skipping the snapshot creation, the image already exists",
				"vm", vm.Name, "snapshot", snapshot.Name)
		} else {
			if !errors.Is(err, ResourceNotFoundError) {
				err = liberr.Wrap(err)
				r.Log.Error(err, "trying to get the image info from the snapshot",
					"vm", vm.Name, "image", image.Name)
				return
			}
			r.Log.Info("creating the volume from snapshot",
				"vm", vm.Name, "snapshot", snapshot.Name)
			_, err = r.createVolumeFromSnapshot(vm, snapshot.ID)
			if err != nil {
				err = liberr.Wrap(err)
				r.Log.Error(err, "trying to create a volume from the VM snapshot",
					"vm", vm.Name, "snapshot", snapshot.Name)
				return

			}
		}
	}
	return
}

func (r *Client) ensureVolumesFromSnapshots(vm *libclient.VM) (err error) {
	var volumesFromSnapshots []libclient.Volume
	if volumesFromSnapshots, err = r.getVolumesFromSnapshots(vm); err != nil {
		err = liberr.Wrap(err)
		return
	}
	for _, volume := range volumesFromSnapshots {
		switch volume.Status {
		case VolumeStatusCreating:
			r.Log.Info("the volume is still being created",
				"vm", vm.Name, "volume", volume.Name, "snapshot", volume.SnapshotID)
		case VolumeStatusUploading:
			r.Log.Info("the volume is still uploading to the image, skipping...",
				"vm", vm.Name, "volume", volume.Name, "snapshot", volume.SnapshotID)
		case VolumeStatusAvailable:
			err = r.ensureImageFromVolume(vm, &volume)
		case VolumeStatusDeleting:
			r.Log.Info("the volume is being deleted",
				"vm", vm.Name, "volume", volume.Name, "snapshot", volume.SnapshotID)
		default:
			err = UnexpectedVolumeStatusError
			r.Log.Error(err, "checking the volume",
				"vm", vm.Name, "volume", volume.Name, "status", volume.Status)
			return
		}
	}
	return
}

func (r *Client) ensureImageFromVolume(vm *libclient.VM, volume *libclient.Volume) (err error) {
	if _, err = r.getImageFromVolume(vm, volume.ID); err != nil {
		if !errors.Is(err, ResourceNotFoundError) {
			err = liberr.Wrap(err)
			r.Log.Error(err, "while trying to get the image from the volume",
				"vm", vm.Name, "volume", volume.Name, "snaphsot", volume.SnapshotID)
			return
		}
		r.Log.Info("creating the image from the volume",
			"vm", vm.Name, "volume", volume.Name, "snapshot", volume.SnapshotID)
		_, err = r.createImageFromVolume(vm, volume.ID)
		if err != nil {
			err = liberr.Wrap(err)
			r.Log.Error(err, "while trying to create the image from the volume",
				"vm", vm.Name, "volume", volume.Name, "snapshot", volume.SnapshotID)
			return
		}
	}
	return
}
