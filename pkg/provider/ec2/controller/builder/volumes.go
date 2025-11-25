package builder

import (
	"fmt"
	"math"

	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	utils "github.com/kubev2v/forklift/pkg/controller/plan/util"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	libitr "github.com/kubev2v/forklift/pkg/lib/itinerary"
	ec2controller "github.com/kubev2v/forklift/pkg/provider/ec2/controller"
	"github.com/kubev2v/forklift/pkg/settings"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

// DataVolumes builds CDI DataVolume specs for an EC2 instance's EBS volumes.
// Creates blank DataVolumes with proper sizing and storage class mapping for each block device.
// Currently creates placeholder DVs; actual volume population uses Ec2VolumePopulator resources.
func (r *Builder) DataVolumes(vmRef ref.Ref, secret *core.Secret, configMap *core.ConfigMap, dvTemplate *cdi.DataVolume, vddkConfigMap *core.ConfigMap) ([]cdi.DataVolume, error) {
	region, err := r.getRegion()
	if err != nil {
		r.log.Error(err, "Failed to get AWS region", "vm", vmRef.Name)
		return nil, liberr.Wrap(err)
	}

	instance := &unstructured.Unstructured{}
	instance.SetUnstructuredContent(map[string]interface{}{"kind": "Instance"})
	err = r.Source.Inventory.Find(instance, vmRef)
	if err != nil {
		return nil, err
	}

	awsInstance, err := ec2controller.GetAWSObject(instance)
	if err != nil {
		return nil, err
	}

	name, _, _ := unstructured.NestedString(awsInstance, "name")
	if name == "" {
		uid, _, _ := unstructured.NestedString(awsInstance, "InstanceId")
		name = uid
	}

	var dataVolumes []cdi.DataVolume

	blockDevices, found, _ := unstructured.NestedSlice(awsInstance, "BlockDeviceMappings")
	if !found {
		return dataVolumes, nil
	}

	for i, devIface := range blockDevices {
		dev, ok := devIface.(map[string]interface{})
		if !ok {
			continue
		}

		volumeID, _, _ := unstructured.NestedString(dev, "Ebs", "VolumeId")

		volume := &unstructured.Unstructured{}
		volume.SetUnstructuredContent(map[string]interface{}{"kind": "Volume"})
		volumeRef := ref.Ref{ID: volumeID}
		err = r.Source.Inventory.Find(volume, volumeRef)
		if err != nil {
			r.log.Error(err, "Failed to get volume", "volumeId", volumeID)
			continue
		}

		awsVolume, err := ec2controller.GetAWSObject(volume)
		if err != nil {
			r.log.Error(err, "Failed to get AWS volume object", "volumeId", volumeID)
			continue
		}

		// Try int64 first (direct access)
		sizeGiB, found, _ := unstructured.NestedInt64(awsVolume, "Size")
		if !found {
			// Try float64 (JSON unmarshaling converts numbers to float64)
			sizeFloat, foundFloat, _ := unstructured.NestedFloat64(awsVolume, "Size")
			if foundFloat {
				sizeGiB = int64(sizeFloat)
				found = true
			}
		}
		if !found {
			sizeGiB = 10
		}

		volumeType, _, _ := unstructured.NestedString(awsVolume, "VolumeType")
		storageClass := r.findStorageMapping(volumeType)

		diskName := fmt.Sprintf("disk%d", i)
		dvName := fmt.Sprintf("%s-%s", name, diskName)

		dv := cdi.DataVolume{
			ObjectMeta: meta.ObjectMeta{
				Name:      dvName,
				Namespace: r.Plan.Spec.TargetNamespace,
				Labels:    r.Labeler.VMLabels(vmRef),
				Annotations: map[string]string{
					"forklift.konveyor.io/disk-source":                 "ec2",
					"forklift.konveyor.io/volume-id":                   volumeID,
					"forklift.konveyor.io/region":                      region,
					"cdi.kubevirt.io/storage.bind.immediate.requested": "true",
				},
			},
			Spec: cdi.DataVolumeSpec{
				Source: &cdi.DataVolumeSource{
					Blank: &cdi.DataVolumeBlankImage{},
				},
				Storage: &cdi.StorageSpec{
					Resources: core.VolumeResourceRequirements{
						Requests: core.ResourceList{
							core.ResourceStorage: resource.MustParse(fmt.Sprintf("%dGi", sizeGiB)),
						},
					},
					StorageClassName: &storageClass,
				},
			},
		}

		dataVolumes = append(dataVolumes, dv)
	}

	return dataVolumes, nil
}

// getVolumeSize retrieves EBS volume size in GiB from inventory, trying volume first then snapshot.
// Falls back to 100 GiB default if both lookups fail. Handles int64/float64 JSON type variations.
func (r *Builder) getVolumeSize(volumeID, snapshotID string) int64 {
	// First, try to get the size from the original volume
	if volumeID != "" {
		volume := &unstructured.Unstructured{}
		volume.SetUnstructuredContent(map[string]interface{}{"kind": "Volume"})
		volumeRef := ref.Ref{ID: volumeID}
		err := r.Source.Inventory.Find(volume, volumeRef)
		if err == nil {
			awsVolume, err := ec2controller.GetAWSObject(volume)
			if err == nil {
				// Try int64 first (direct access)
				sizeGiB, found, _ := unstructured.NestedInt64(awsVolume, "Size")
				if !found {
					// Try float64 (JSON unmarshaling converts numbers to float64)
					sizeFloat, foundFloat, _ := unstructured.NestedFloat64(awsVolume, "Size")
					if foundFloat {
						sizeGiB = int64(sizeFloat)
						found = true
					}
				}
				if found {
					r.log.Info("Volume size from AWS inventory",
						"volumeId", volumeID,
						"sizeGiB", sizeGiB)
					return sizeGiB
				}
				// Log available keys for debugging
				keys := make([]string, 0, len(awsVolume))
				for k := range awsVolume {
					keys = append(keys, k)
				}
				r.log.V(1).Info("Volume Size field not found in inventory, trying snapshot",
					"volumeId", volumeID,
					"availableKeys", keys)
			} else {
				r.log.V(1).Info("Failed to get AWS volume object, trying snapshot", "volumeId", volumeID, "error", err)
			}
		} else {
			r.log.V(1).Info("Failed to find volume in inventory, trying snapshot", "volumeId", volumeID, "error", err)
		}
	}

	// Fall back to getting size from snapshot
	if snapshotID != "" {
		snapshot := &unstructured.Unstructured{}
		snapshot.SetUnstructuredContent(map[string]interface{}{"kind": "Snapshot"})
		snapshotRef := ref.Ref{ID: snapshotID}
		err := r.Source.Inventory.Find(snapshot, snapshotRef)
		if err == nil {
			awsSnapshot, err := ec2controller.GetAWSObject(snapshot)
			if err == nil {
				// Try int64 first (direct access)
				sizeGiB, found, _ := unstructured.NestedInt64(awsSnapshot, "VolumeSize")
				if !found {
					// Try float64 (JSON unmarshaling converts numbers to float64)
					sizeFloat, foundFloat, _ := unstructured.NestedFloat64(awsSnapshot, "VolumeSize")
					if foundFloat {
						sizeGiB = int64(sizeFloat)
						found = true
					}
				}
				if found {
					r.log.Info("Snapshot size from AWS inventory",
						"snapshotId", snapshotID,
						"sizeGiB", sizeGiB)
					return sizeGiB
				}
				r.log.V(1).Info("Snapshot VolumeSize field not found in inventory", "snapshotId", snapshotID)
			} else {
				r.log.V(1).Info("Failed to get AWS snapshot object", "snapshotId", snapshotID, "error", err)
			}
		} else {
			r.log.V(1).Info("Failed to find snapshot in inventory", "snapshotId", snapshotID, "error", err)
		}
	}

	// Default to 100 GiB if both lookups fail
	r.log.Error(nil, "Failed to get volume or snapshot size from inventory, using default 100 GiB",
		"volumeId", volumeID,
		"snapshotId", snapshotID)
	return 100
}

// calculatePVCSize calculates PVC size by adding filesystem/block overhead to the aligned volume size.
// Filesystem mode adds percentage overhead, block mode adds fixed bytes for metadata.
func (r *Builder) calculatePVCSize(volumeSizeBytes int64, volumeMode *core.PersistentVolumeMode) *resource.Quantity {
	alignedSize := utils.RoundUp(volumeSizeBytes, utils.DefaultAlignBlockSize)

	var sizeWithOverhead int64
	if *volumeMode == core.PersistentVolumeFilesystem {
		sizeWithOverhead = int64(math.Ceil(float64(alignedSize) / (1 - float64(settings.Settings.FileSystemOverhead)/100)))
	} else {
		sizeWithOverhead = alignedSize + settings.Settings.BlockOverhead
	}

	r.log.V(1).Info("Calculated PVC size with overhead",
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

// ResolveDataVolumeIdentifier extracts the EBS volume ID from a DataVolume's annotations.
// Used to map DataVolumes back to their source EBS volumes during migration.
func (r *Builder) ResolveDataVolumeIdentifier(dv *cdi.DataVolume) string {
	if dv.ObjectMeta.Annotations != nil {
		return dv.ObjectMeta.Annotations["forklift.konveyor.io/volume-id"]
	}
	return ""
}

// ResolvePersistentVolumeClaimIdentifier extracts the EBS volume ID from a PVC's annotations.
// Enables tracking which PVC corresponds to which source EBS volume.
func (r *Builder) ResolvePersistentVolumeClaimIdentifier(pvc *core.PersistentVolumeClaim) string {
	if pvc.ObjectMeta.Annotations != nil {
		return pvc.ObjectMeta.Annotations["forklift.konveyor.io/volume-id"]
	}
	return ""
}

// SetPopulatorDataSourceLabels adds VM and provider labels to PVCs for resource tracking.
// Labels enable finding PVCs associated with a specific VM or provider.
func (r *Builder) SetPopulatorDataSourceLabels(vmRef ref.Ref, pvcs []*core.PersistentVolumeClaim) (err error) {
	instance := &unstructured.Unstructured{}
	instance.SetUnstructuredContent(map[string]interface{}{"kind": "Instance"})
	err = r.Source.Inventory.Find(instance, vmRef)
	if err != nil {
		return
	}

	for _, pvc := range pvcs {
		if pvc.Labels == nil {
			pvc.Labels = make(map[string]string)
		}
		pvc.Labels["forklift.konveyor.io/vm"] = vmRef.ID
		pvc.Labels["forklift.konveyor.io/provider"] = r.Source.Provider.Name
	}

	return
}

// SupportsVolumePopulators returns true as EC2 uses Ec2VolumePopulator for disk migration.
// Volume populators create EBS volumes from snapshots asynchronously.
func (r *Builder) SupportsVolumePopulators() bool {
	return true
}

// PopulatorTransferredBytes estimates volume population progress based on PVC phase and conditions.
// Returns percentage-based progress estimates: Pending=20-60%, Resizing=50-75%, Bound=100%.
func (r *Builder) PopulatorTransferredBytes(pvc *core.PersistentVolumeClaim) (transferredBytes int64, err error) {
	pvcSize := pvc.Spec.Resources.Requests[core.ResourceStorage]
	totalBytes := pvcSize.Value()

	r.log.V(2).Info("Checking PVC transfer progress",
		"pvc", pvc.Name,
		"phase", pvc.Status.Phase,
		"totalBytes", totalBytes)

	switch pvc.Status.Phase {
	case core.ClaimBound:
		transferredBytes = totalBytes
		r.log.V(2).Info("PVC is bound, transfer complete",
			"pvc", pvc.Name,
			"transferredBytes", transferredBytes)
		return

	case core.ClaimPending:
		for _, condition := range pvc.Status.Conditions {
			switch condition.Type {
			case core.PersistentVolumeClaimResizing:
				if condition.Status == core.ConditionTrue {
					transferredBytes = totalBytes / 2
					r.log.V(2).Info("PVC is resizing",
						"pvc", pvc.Name,
						"transferredBytes", transferredBytes)
					return
				}

			case "FileSystemResizePending":
				if condition.Status == core.ConditionTrue {
					transferredBytes = (totalBytes * 3) / 4
					r.log.V(2).Info("PVC filesystem resize pending",
						"pvc", pvc.Name,
						"transferredBytes", transferredBytes)
					return
				}
			}
		}

		if pvc.Spec.VolumeName != "" {
			transferredBytes = (totalBytes * 6) / 10
			r.log.V(2).Info("PVC has volume assigned, creation in progress",
				"pvc", pvc.Name,
				"volumeName", pvc.Spec.VolumeName,
				"transferredBytes", transferredBytes)
			return
		}

		transferredBytes = totalBytes / 5
		r.log.V(2).Info("PVC is pending, waiting for volume",
			"pvc", pvc.Name,
			"transferredBytes", transferredBytes)
		return

	case core.ClaimLost:
		err = fmt.Errorf("PVC %s is in Lost state", pvc.Name)
		r.log.Error(err, "PVC lost", "pvc", pvc.Name)
		return

	default:
		transferredBytes = 0
		r.log.V(2).Info("PVC in unknown state",
			"pvc", pvc.Name,
			"phase", pvc.Status.Phase)
		return
	}
}

// GetPopulatorTaskName returns the volume ID annotation from a PVC for progress tracking.
// The volume ID serves as the task identifier in the migration pipeline.
func (r *Builder) GetPopulatorTaskName(pvc *core.PersistentVolumeClaim) (taskName string, err error) {
	if pvc.Annotations != nil {
		taskName = pvc.Annotations["forklift.konveyor.io/volume-id"]
	}
	return
}

// Tasks creates a progress tracking task for each EBS volume attached to the instance.
// Each task uses the volume size in MB as the progress total for UI display.
func (r *Builder) Tasks(vmRef ref.Ref) (tasks []*plan.Task, err error) {
	instance := &unstructured.Unstructured{}
	instance.SetUnstructuredContent(map[string]interface{}{"kind": "Instance"})
	err = r.Source.Inventory.Find(instance, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}

	awsInstance, err := ec2controller.GetAWSObject(instance)
	if err != nil {
		return tasks, err
	}

	blockDevices, found, _ := unstructured.NestedSlice(awsInstance, "BlockDeviceMappings")
	if !found {
		blockDevices, found, _ = unstructured.NestedSlice(awsInstance, "BlockDeviceMappings")
	}
	if !found {
		return tasks, nil
	}

	for _, devIface := range blockDevices {
		dev, ok := devIface.(map[string]interface{})
		if !ok {
			continue
		}

		volumeID, _, _ := unstructured.NestedString(dev, "Ebs", "VolumeId")

		volume := &unstructured.Unstructured{}
		volume.SetUnstructuredContent(map[string]interface{}{"kind": "Volume"})
		volumeRef := ref.Ref{ID: volumeID}
		err = r.Source.Inventory.Find(volume, volumeRef)
		if err != nil {
			r.log.Error(err, "Failed to get volume", "volumeId", volumeID)
			continue
		}

		awsVolume, err := ec2controller.GetAWSObject(volume)
		if err != nil {
			r.log.Error(err, "Failed to get AWS volume object", "volumeId", volumeID)
			continue
		}

		// Try int64 first (direct access)
		sizeGiB, found, _ := unstructured.NestedInt64(awsVolume, "Size")
		if !found {
			// Try float64 (JSON unmarshaling converts numbers to float64)
			sizeFloat, foundFloat, _ := unstructured.NestedFloat64(awsVolume, "Size")
			if foundFloat {
				sizeGiB = int64(sizeFloat)
				found = true
			}
		}
		if !found {
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
