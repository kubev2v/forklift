package vsphere

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planbase "github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	"github.com/kubev2v/forklift/pkg/controller/provider/model/vsphere"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/vsphere"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	core "k8s.io/api/core/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// vSphere datastore path regex patterns
var (
	// Matches vSphere datastore format: [datastore]<opt-space>path
	datastorePattern = regexp.MustCompile(`^\[([^\]]+)\]\s*(.*)$`)
	// Matches filename at end of path (after last / or \)
	filenamePattern = regexp.MustCompile(`[^/\\]*$`)
)

// extractDiskFileName extracts the filename from a vSphere disk file path.
// Input:  "[datastore1] folder/vm-disk.vmdk"
// Output: "vm-disk.vmdk"
func extractDiskFileName(diskPath string) string {
	if diskPath == "" {
		return ""
	}

	path := diskPath

	// Handle vSphere datastore format: [datastore] path
	if matches := datastorePattern.FindStringSubmatch(diskPath); len(matches) == 3 {
		path = matches[2] // Extract the path part after "[datastore]"
		path = strings.TrimSpace(path)
	}

	// If path ends with separator, it's a directory - return empty
	if strings.HasSuffix(path, "/") || strings.HasSuffix(path, "\\") {
		return ""
	}

	// Extract filename using regex (everything after last / or \)
	filename := filenamePattern.FindString(path)
	return filename
}

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

// stringifyWithQuotes formats the slice with comas between the items and quotes around each item
// Example [disk1, disk2, disk3] -> 'disk1', 'disk2', 'disk3'
func stringifyWithQuotes(s []string) string {
	return fmt.Sprintf("'%s'", strings.Join(s, "', '"))
}

// listShareablePVCs returns all non-terminating shareable PVCs in the namespace.
func listShareablePVCs(c client.Client, targetNamespace string) (pvcs []*core.PersistentVolumeClaim, err error) {
	pvcsList := &core.PersistentVolumeClaimList{}
	err = c.List(
		context.TODO(),
		pvcsList,
		&client.ListOptions{
			LabelSelector: k8slabels.SelectorFromSet(map[string]string{
				Shareable: "true",
			}),
			Namespace: targetNamespace,
		},
	)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	for i := range pvcsList.Items {
		pvc := &pvcsList.Items[i]
		if pvc.DeletionTimestamp == nil {
			pvcs = append(pvcs, pvc)
		}
	}
	return
}

// getDiskSharedPVC finds a shareable PVC for the given disk. It prefers a PVC
// that belongs to the same plan (matched by the "plan" label); if none is
// found it falls back to any shareable PVC with a matching disk source.
func getDiskSharedPVC(disk vsphere.Disk, pvcs []*core.PersistentVolumeClaim, planID string) *core.PersistentVolumeClaim {
	diskSource := baseVolume(disk.File, false)
	var fallback *core.PersistentVolumeClaim
	for _, pvc := range pvcs {
		if pvc.Annotations[planbase.AnnDiskSource] != diskSource {
			continue
		}
		if planID != "" && pvc.Labels["plan"] == planID {
			return pvc
		}
		if fallback == nil {
			fallback = pvc
		}
	}
	return fallback
}

// findSharedPVCs returns the shareable PVCs that match the VM's shared disks.
// For each disk it prefers a PVC belonging to the same plan (by planID label),
// falling back to any matching shareable PVC.
func findSharedPVCs(client client.Client, vm *model.VM, targetNamespace, planID string) (pvcs []*core.PersistentVolumeClaim, missingDiskPVCs []vsphere.Disk, err error) {
	allPvcs, err := listShareablePVCs(client, targetNamespace)
	if err != nil {
		return
	}

	for _, disk := range vm.Disks {
		if !disk.Shared {
			continue
		}
		pvc := getDiskSharedPVC(disk, allPvcs, planID)
		if pvc != nil {
			pvcs = append(pvcs, pvc)
		} else {
			missingDiskPVCs = append(missingDiskPVCs, disk)
		}
	}
	return pvcs, missingDiskPVCs, err
}

func useCompatibilityModeBus(plan *api.Plan) bool {
	return plan.Spec.SkipGuestConversion && plan.Spec.UseCompatibilityMode
}
