package vsphere

import (
	"context"

	planbase "github.com/konveyor/forklift-controller/pkg/controller/plan/adapter/base"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/model/vsphere"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/web/vsphere"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	core "k8s.io/api/core/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Return PersistentVolumeClaims associated with a VM.
func getDisksPvc(disk vsphere.Disk, pvcs []*core.PersistentVolumeClaim, warm bool) *core.PersistentVolumeClaim {
	for _, pvc := range pvcs {
		if pvc.Annotations[planbase.AnnDiskSource] == baseVolume(disk.File, warm) {
			return pvc
		}
	}
	return nil
}

func baseVolume(fileName string, warm bool) string {
	if warm {
		// for warm migrations, we return the very first volume of the disk
		// as the base volume and CBT will be used to transfer later changes
		return trimBackingFileName(fileName)
	} else {
		// for cold migrations, we return the latest volume as the base,
		// e.g., my-vm/disk-name-000015.vmdk, since we should transfer
		// only its state
		// note that this setting is insignificant when we use virt-v2v on
		// el9 since virt-v2v doesn't receive the volume to transfer - we
		// only need this to be consistent for correlating disks with PVCs
		return fileName
	}
}

// Trims the snapshot suffix from a disk backing file name if there is one.
//
//	Example:
//	Input: 	[datastore13] my-vm/disk-name-000015.vmdk
//	Output: [datastore13] my-vm/disk-name.vmdk
func trimBackingFileName(fileName string) string {
	return backingFilePattern.ReplaceAllString(fileName, ".vmdk")
}

// Return all shareable PVCs
func listShareablePVCs(c client.Client) (pvcs []*core.PersistentVolumeClaim, err error) {
	pvcsList := &core.PersistentVolumeClaimList{}
	err = c.List(
		context.TODO(),
		pvcsList,
		&client.ListOptions{
			LabelSelector: k8slabels.SelectorFromSet(map[string]string{
				Shareable: "true",
			}),
		},
	)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	pvcs = make([]*core.PersistentVolumeClaim, len(pvcsList.Items))
	for i, pvc := range pvcsList.Items {
		// loopvar
		copyPvc := pvc
		pvcs[i] = &copyPvc
	}

	return
}

// Return PersistentVolumeClaims and disks associated with a VM.
func findSharedPVCs(c client.Client, vm *model.VM) (pvcs []*core.PersistentVolumeClaim, missingDiskPVCs []vsphere.Disk, err error) {
	allPvcs, err := listShareablePVCs(c)
	if err != nil {
		return
	}

	for _, disk := range vm.Disks {
		if !disk.Shared {
			continue
		}
		// Warm migration disable as the shared disks can't be migrated with warm migration
		pvc := getDisksPvc(disk, allPvcs, false)
		if pvc != nil {
			pvcs = append(pvcs, pvc)
		} else {
			missingDiskPVCs = append(missingDiskPVCs, disk)
		}
	}
	return pvcs, missingDiskPVCs, err
}
