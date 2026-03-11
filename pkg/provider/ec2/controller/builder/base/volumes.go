package base

import (
	"fmt"
	"math"

	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	utils "github.com/kubev2v/forklift/pkg/controller/plan/util"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	libitr "github.com/kubev2v/forklift/pkg/lib/itinerary"
	"github.com/kubev2v/forklift/pkg/provider/ec2/controller/inventory"
	"github.com/kubev2v/forklift/pkg/settings"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// EBSCSIDriver is the CSI driver name for AWS EBS volumes.
	EBSCSIDriver = "ebs.csi.aws.com"
)

// VolumeInfo contains information about an EBS volume for PV/PVC creation.
type VolumeInfo struct {
	// EBSVolumeID is the AWS EBS volume ID (e.g., vol-0123456789abcdef0)
	EBSVolumeID string
	// OriginalVolumeID is the original source volume ID before snapshot
	OriginalVolumeID string
	// SnapshotID is the snapshot ID used to create this volume
	SnapshotID string
	// SizeGiB is the volume size in GiB
	SizeGiB int64
	// VolumeType is the EBS volume type (e.g., gp3, gp2, io1)
	VolumeType string
	// AvailabilityZone is the AZ where the volume was created
	AvailabilityZone string
}

// GetVolumeSize retrieves EBS volume size in GiB from inventory, trying volume first then snapshot.
// Falls back to 100 GiB default if both lookups fail.
func (r *Base) GetVolumeSize(volumeID, snapshotID string) int64 {
	return r.getVolumeSize(volumeID, snapshotID)
}

// GetVolumeType retrieves the EBS volume type from inventory.
// Returns an empty string if the volume type cannot be determined.
func (r *Base) GetVolumeType(volumeID string) string {
	return inventory.GetVolumeType(r.Source.Inventory, volumeID)
}

func (r *Base) getVolumeSize(volumeID, snapshotID string) int64 {
	if volumeID != "" {
		sizeGiB := inventory.GetVolumeSize(r.Source.Inventory, volumeID)
		if sizeGiB > 0 {
			r.Log.Info("Volume size from AWS inventory",
				"volumeId", volumeID,
				"sizeGiB", sizeGiB)
			return sizeGiB
		}
		r.Log.V(1).Info("Volume not found in inventory, trying snapshot", "volumeId", volumeID)
	}

	if snapshotID != "" {
		r.Log.V(1).Info("Snapshots not currently in inventory, using default 100 GiB", "snapshotId", snapshotID)
	}

	r.Log.Error(nil, "Failed to get volume or snapshot size from inventory, using default 100 GiB",
		"volumeId", volumeID,
		"snapshotId", snapshotID)
	return 100
}

// CalculatePVCSize calculates PVC size by adding filesystem/block overhead to the aligned volume size.
func (r *Base) CalculatePVCSize(volumeSizeBytes int64, volumeMode *core.PersistentVolumeMode) *resource.Quantity {
	alignedSize := utils.RoundUp(volumeSizeBytes, utils.DefaultAlignBlockSize)

	var sizeWithOverhead int64
	if *volumeMode == core.PersistentVolumeFilesystem {
		sizeWithOverhead = int64(math.Ceil(float64(alignedSize) / (1 - float64(settings.Settings.FileSystemOverhead)/100)))
	} else {
		sizeWithOverhead = alignedSize + settings.Settings.BlockOverhead
	}

	r.Log.V(1).Info("Calculated PVC size with overhead",
		"originalBytes", volumeSizeBytes,
		"originalGiB", volumeSizeBytes/(1024*1024*1024),
		"alignedBytes", alignedSize,
		"withOverheadBytes", sizeWithOverhead,
		"withOverheadGiB", sizeWithOverhead/(1024*1024*1024),
		"volumeMode", *volumeMode,
		"blockOverhead", settings.Settings.BlockOverhead,
	)

	return resource.NewQuantity(sizeWithOverhead, resource.BinarySI)
}

// ResolvePersistentVolumeClaimIdentifier extracts the EBS volume ID from a PVC's annotations.
func (r *Base) ResolvePersistentVolumeClaimIdentifier(pvc *core.PersistentVolumeClaim) string {
	if pvc.ObjectMeta.Annotations != nil {
		return pvc.ObjectMeta.Annotations["forklift.konveyor.io/volume-id"]
	}
	return ""
}

// PopulatorVolumes is a no-op for EC2.
func (r *Base) PopulatorVolumes(vmRef ref.Ref, annotations map[string]string, secretName string) ([]*core.PersistentVolumeClaim, error) {
	return nil, nil
}

// SetPopulatorDataSourceLabels is a no-op for EC2.
func (r *Base) SetPopulatorDataSourceLabels(vmRef ref.Ref, pvcs []*core.PersistentVolumeClaim) (err error) {
	return nil
}

// SupportsVolumePopulators returns false as EC2 uses direct volume creation.
func (r *Base) SupportsVolumePopulators() bool {
	return false
}

// PopulatorTransferredBytes is a no-op for EC2.
func (r *Base) PopulatorTransferredBytes(pvc *core.PersistentVolumeClaim) (transferredBytes int64, err error) {
	if pvc.Status.Phase == core.ClaimBound {
		pvcSize := pvc.Spec.Resources.Requests[core.ResourceStorage]
		return pvcSize.Value(), nil
	}
	return 0, nil
}

// GetPopulatorTaskName is a no-op for EC2.
func (r *Base) GetPopulatorTaskName(pvc *core.PersistentVolumeClaim) (taskName string, err error) {
	if pvc.Annotations != nil {
		taskName = pvc.Annotations["forklift.konveyor.io/original-volume-id"]
	}
	return
}

// Tasks creates a progress tracking task for each EBS volume attached to the instance.
func (r *Base) Tasks(vmRef ref.Ref) (tasks []*plan.Task, err error) {
	awsInstance, err := inventory.GetAWSInstance(r.Source.Inventory, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}

	devices, found := inventory.GetBlockDevices(awsInstance)
	if !found {
		return tasks, nil
	}

	for _, dev := range devices {
		volumeID := inventory.ExtractEBSVolumeID(dev)
		if volumeID == "" {
			continue
		}

		sizeGiB := inventory.GetVolumeSize(r.Source.Inventory, volumeID)
		if sizeGiB == 0 {
			sizeGiB = 10
		}
		sizeMB := sizeGiB * 1024

		task := &plan.Task{
			Name: volumeID,
			Progress: libitr.Progress{
				Total: sizeMB,
			},
			Annotations: map[string]string{
				"unit": "MB",
			},
		}
		tasks = append(tasks, task)
	}

	return
}

// BuildPersistentVolume creates a PV spec with CSI volume source pointing to an EBS volume.
func (r *Base) BuildPersistentVolume(vmRef ref.Ref, volumeInfo *VolumeInfo, pvcName, pvcNamespace string) (*core.PersistentVolume, error) {
	r.Log.Info("Building PersistentVolume for EBS volume",
		"vm", vmRef.Name,
		"ebsVolumeID", volumeInfo.EBSVolumeID,
		"pvcName", pvcName,
		"pvcNamespace", pvcNamespace)

	storageClass := r.FindStorageMapping(volumeInfo.VolumeType)
	blockMode := core.PersistentVolumeBlock

	pvLabels := r.Labeler.VMLabels(vmRef)
	pvLabels["forklift.konveyor.io/ebs-volume-id"] = volumeInfo.EBSVolumeID

	pv := &core.PersistentVolume{
		ObjectMeta: meta.ObjectMeta{
			GenerateName: fmt.Sprintf("pv-%s-", pvcName),
			Labels:       pvLabels,
			Annotations: map[string]string{
				"forklift.konveyor.io/ebs-volume-id":      volumeInfo.EBSVolumeID,
				"forklift.konveyor.io/original-volume-id": volumeInfo.OriginalVolumeID,
				"forklift.konveyor.io/snapshot-id":        volumeInfo.SnapshotID,
			},
		},
		Spec: core.PersistentVolumeSpec{
			Capacity: core.ResourceList{
				core.ResourceStorage: resource.MustParse(fmt.Sprintf("%dGi", volumeInfo.SizeGiB)),
			},
			AccessModes: []core.PersistentVolumeAccessMode{
				core.ReadWriteOnce,
			},
			VolumeMode:                    &blockMode,
			PersistentVolumeReclaimPolicy: core.PersistentVolumeReclaimDelete,
			StorageClassName:              storageClass,
			PersistentVolumeSource: core.PersistentVolumeSource{
				CSI: &core.CSIPersistentVolumeSource{
					Driver:       EBSCSIDriver,
					VolumeHandle: volumeInfo.EBSVolumeID,
					VolumeAttributes: map[string]string{
						"storage.kubernetes.io/csiProvisionerIdentity": EBSCSIDriver,
					},
				},
			},
			ClaimRef: &core.ObjectReference{
				Kind:      "PersistentVolumeClaim",
				Namespace: pvcNamespace,
				Name:      pvcName,
			},
		},
	}

	r.Log.Info("Built PersistentVolume spec",
		"vm", vmRef.Name,
		"pvGenerateName", pv.GenerateName,
		"ebsVolumeID", volumeInfo.EBSVolumeID,
		"storageClass", storageClass,
		"sizeGiB", volumeInfo.SizeGiB)

	return pv, nil
}

// BuildDirectPVC creates a PVC spec that will bind to a pre-created PV.
func (r *Base) BuildDirectPVC(vmRef ref.Ref, volumeInfo *VolumeInfo, index int) (*core.PersistentVolumeClaim, error) {
	r.Log.Info("Building direct PVC for EBS volume",
		"vm", vmRef.Name,
		"ebsVolumeID", volumeInfo.EBSVolumeID,
		"originalVolumeID", volumeInfo.OriginalVolumeID,
		"index", index)

	storageClass := r.FindStorageMapping(volumeInfo.VolumeType)
	blockMode := core.PersistentVolumeBlock

	volumeSizeBytes := volumeInfo.SizeGiB * 1024 * 1024 * 1024
	pvcSize := r.CalculatePVCSize(volumeSizeBytes, &blockMode)

	pvcLabels := r.Labeler.VMLabels(vmRef)
	pvcLabels["forklift.konveyor.io/volume-id"] = volumeInfo.OriginalVolumeID

	pvcAnnotations := map[string]string{
		"forklift.konveyor.io/original-volume-id": volumeInfo.OriginalVolumeID,
		"forklift.konveyor.io/ebs-volume-id":      volumeInfo.EBSVolumeID,
		"forklift.konveyor.io/snapshot-id":        volumeInfo.SnapshotID,
		"forklift.konveyor.io/disk-index":         fmt.Sprintf("%d", index),
	}

	pvc := &core.PersistentVolumeClaim{
		ObjectMeta: meta.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-disk-", vmRef.Name),
			Namespace:    r.Plan.Spec.TargetNamespace,
			Labels:       pvcLabels,
			Annotations:  pvcAnnotations,
		},
		Spec: core.PersistentVolumeClaimSpec{
			AccessModes: []core.PersistentVolumeAccessMode{
				core.ReadWriteOnce,
			},
			VolumeMode: &blockMode,
			Resources: core.VolumeResourceRequirements{
				Requests: core.ResourceList{
					core.ResourceStorage: *pvcSize,
				},
			},
			StorageClassName: &storageClass,
		},
	}

	r.Log.Info("Built direct PVC spec",
		"vm", vmRef.Name,
		"pvcGenerateName", pvc.GenerateName,
		"ebsVolumeID", volumeInfo.EBSVolumeID,
		"storageClass", storageClass,
		"pvcSize", pvcSize.String())

	return pvc, nil
}
