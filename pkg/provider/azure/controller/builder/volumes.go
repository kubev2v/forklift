package builder

import (
	"context"
	"fmt"
	"math"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	utils "github.com/kubev2v/forklift/pkg/controller/plan/util"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	libitr "github.com/kubev2v/forklift/pkg/lib/itinerary"
	"github.com/kubev2v/forklift/pkg/provider/azure"
	"github.com/kubev2v/forklift/pkg/provider/azure/controller/inventory"
	"github.com/kubev2v/forklift/pkg/settings"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	AzureCSIDriver = "disk.csi.azure.com"
)

func (r *Builder) Tasks(vmRef ref.Ref) (tasks []*plan.Task, err error) {
	azureVM, err := inventory.GetAzureVM(r.Source.Inventory, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}

	disks := inventory.GetManagedDisks(azureVM)
	for i, disk := range disks {
		if disk.SizeGB == 0 {
			err = fmt.Errorf("disk %d (%s): unable to determine size", i, disk.Name)
			return
		}
		sizeMB := int64(disk.SizeGB) * 1024

		task := &plan.Task{
			Name: disk.ID,
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

// resolveSnapshotClassName returns the VolumeSnapshotClass to use.
// Priority: provider setting > auto-discover by driver name.
func (r *Builder) resolveSnapshotClassName() (string, error) {
	if name := r.Source.Provider.Spec.Settings[api.AzureSnapshotClass]; name != "" {
		return name, nil
	}
	vscList := &snapshotv1.VolumeSnapshotClassList{}
	err := r.Destination.Client.List(context.TODO(), vscList, &client.ListOptions{})
	if err != nil {
		return "", liberr.Wrap(err)
	}
	for _, vsc := range vscList.Items {
		if vsc.Driver == AzureCSIDriver {
			r.log.Info("Auto-discovered VolumeSnapshotClass", "name", vsc.Name)
			return vsc.Name, nil
		}
	}
	return "", fmt.Errorf("no VolumeSnapshotClass found for driver %s", AzureCSIDriver)
}

func (r *Builder) BuildVolumeSnapshotContent(vmRef ref.Ref, snapshotResourceID string, diskIndex int) (*snapshotv1.VolumeSnapshotContent, error) {
	labels := r.Labeler.MigrationVMLabels(vmRef)
	labels[azure.LabelDiskIndex] = fmt.Sprintf("%d", diskIndex)

	snapshotClassName, err := r.resolveSnapshotClassName()
	if err != nil {
		return nil, err
	}
	deletionPolicy := snapshotv1.VolumeSnapshotContentDelete

	vscName := fmt.Sprintf("%s-snapcontent-%d", vmRef.Name, diskIndex)
	vsc := &snapshotv1.VolumeSnapshotContent{
		ObjectMeta: meta.ObjectMeta{
			Name:        vscName,
			Labels:      labels,
			Annotations: map[string]string{azure.AnnSourceID: r.vmARMID(vmRef.Name)},
		},
		Spec: snapshotv1.VolumeSnapshotContentSpec{
			DeletionPolicy: deletionPolicy,
			Driver:         AzureCSIDriver,
			Source: snapshotv1.VolumeSnapshotContentSource{
				SnapshotHandle: &snapshotResourceID,
			},
			VolumeSnapshotRef: core.ObjectReference{
				Kind:      "VolumeSnapshot",
				Namespace: r.Plan.Spec.TargetNamespace,
				Name:      fmt.Sprintf("%s-snap-%d", vmRef.Name, diskIndex),
			},
			VolumeSnapshotClassName: &snapshotClassName,
		},
	}

	return vsc, nil
}

func (r *Builder) BuildVolumeSnapshot(vmRef ref.Ref, diskIndex int) (*snapshotv1.VolumeSnapshot, error) {
	labels := r.Labeler.MigrationVMLabels(vmRef)
	labels[azure.LabelDiskIndex] = fmt.Sprintf("%d", diskIndex)

	snapshotClassName, err := r.resolveSnapshotClassName()
	if err != nil {
		return nil, err
	}

	vs := &snapshotv1.VolumeSnapshot{
		ObjectMeta: meta.ObjectMeta{
			Name:        fmt.Sprintf("%s-snap-%d", vmRef.Name, diskIndex),
			Namespace:   r.Plan.Spec.TargetNamespace,
			Labels:      labels,
			Annotations: map[string]string{azure.AnnSourceID: r.vmARMID(vmRef.Name)},
		},
		Spec: snapshotv1.VolumeSnapshotSpec{
			Source: snapshotv1.VolumeSnapshotSource{
				VolumeSnapshotContentName: ptr.To(fmt.Sprintf("%s-snapcontent-%d", vmRef.Name, diskIndex)),
			},
			VolumeSnapshotClassName: &snapshotClassName,
		},
	}

	return vs, nil
}

func (r *Builder) BuildPVC(vmRef ref.Ref, diskSizeGiB int64, diskSKU string, diskIndex int, diskID string) (*core.PersistentVolumeClaim, error) {
	storageClass := r.findStorageMapping(diskSKU)
	blockMode := core.PersistentVolumeBlock

	volumeSizeBytes := diskSizeGiB * 1024 * 1024 * 1024
	pvcSize := r.calculatePVCSize(volumeSizeBytes, &blockMode)

	pvcLabels := r.Labeler.MigrationVMLabels(vmRef)
	pvcLabels[azure.LabelDiskIndex] = fmt.Sprintf("%d", diskIndex)

	snapshotName := fmt.Sprintf("%s-snap-%d", vmRef.Name, diskIndex)
	snapshotAPIGroup := "snapshot.storage.k8s.io"

	pvc := &core.PersistentVolumeClaim{
		ObjectMeta: meta.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-disk-%d-", vmRef.Name, diskIndex),
			Namespace:    r.Plan.Spec.TargetNamespace,
			Labels:       pvcLabels,
			Annotations: map[string]string{
				azure.AnnDiskIndex:  fmt.Sprintf("%d", diskIndex),
				azure.AnnSourceID:   r.vmARMID(vmRef.Name),
				azure.AnnDiskSource: diskID,
			},
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
			DataSource: &core.TypedLocalObjectReference{
				APIGroup: &snapshotAPIGroup,
				Kind:     "VolumeSnapshot",
				Name:     snapshotName,
			},
		},
	}

	return pvc, nil
}

func (r *Builder) calculatePVCSize(volumeSizeBytes int64, volumeMode *core.PersistentVolumeMode) *resource.Quantity {
	alignedSize := utils.RoundUp(volumeSizeBytes, utils.DefaultAlignBlockSize)

	var sizeWithOverhead int64
	if *volumeMode == core.PersistentVolumeFilesystem {
		sizeWithOverhead = int64(math.Ceil(float64(alignedSize) / (1 - float64(settings.Settings.FileSystemOverhead)/100)))
	} else {
		sizeWithOverhead = alignedSize + settings.Settings.BlockOverhead
	}

	return resource.NewQuantity(sizeWithOverhead, resource.BinarySI)
}

func (r *Builder) ResolvePersistentVolumeClaimIdentifier(pvc *core.PersistentVolumeClaim) string {
	if pvc.ObjectMeta.Labels != nil {
		return pvc.ObjectMeta.Labels[azure.LabelDiskIndex]
	}
	return ""
}

func (r *Builder) PopulatorVolumes(vmRef ref.Ref, annotations map[string]string, secretName string) ([]*core.PersistentVolumeClaim, error) {
	return nil, nil
}

func (r *Builder) SetPopulatorDataSourceLabels(vmRef ref.Ref, pvcs []*core.PersistentVolumeClaim) (err error) {
	return nil
}

func (r *Builder) SupportsVolumePopulators() bool {
	return false
}

func (r *Builder) PopulatorTransferredBytes(pvc *core.PersistentVolumeClaim) (transferredBytes int64, err error) {
	if pvc.Status.Phase == core.ClaimBound {
		pvcSize := pvc.Spec.Resources.Requests[core.ResourceStorage]
		return pvcSize.Value(), nil
	}
	return 0, nil
}

func (r *Builder) GetPopulatorTaskName(pvc *core.PersistentVolumeClaim) (taskName string, err error) {
	if pvc.Labels != nil {
		taskName = pvc.Labels[azure.LabelDiskIndex]
	}
	return
}

func (r *Builder) vmARMID(vmName string) string {
	subscriptionID := string(r.Source.Secret.Data["subscriptionId"])
	resourceGroup := string(r.Source.Secret.Data["resourceGroup"])
	return fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/virtualMachines/%s",
		subscriptionID, resourceGroup, vmName)
}
