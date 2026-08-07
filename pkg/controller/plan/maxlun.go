package plan

import (
	"errors"
	"fmt"
	"strings"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"github.com/kubev2v/forklift/pkg/controller/provider/web"
	vsphere "github.com/kubev2v/forklift/pkg/controller/provider/web/vsphere"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	"github.com/kubev2v/forklift/pkg/settings"
)

const (
	DiskMaxLUNTooLow = "DiskMaxLUNTooLow"

	// Default ESXi Disk.MaxLUN (LUN IDs 0-1023). Copy-offload README recommends ≥2048.
	defaultDiskMaxLUN = 1024
)

// validateDiskMaxLUN sets a non-blocking warning when a VSphere XCOPY plan may
// exceed the default ESXi Disk.MaxLUN limit (existing SCSI disks + planned disks).
func (r *Reconciler) validateDiskMaxLUN(plan *api.Plan) error {
	plan.Status.DeleteCondition(DiskMaxLUNTooLow)

	if plan.Referenced.Provider.Source == nil || plan.Referenced.Provider.Source.Type() != api.VSphere {
		return nil
	}
	if !settings.Settings.Features.CopyOffload || !r.planUsesVSphereXcopyPopulator(plan) {
		return nil
	}
	if plan.Referenced.Map.Storage == nil {
		return nil
	}

	inventory, err := web.NewClient(plan.Referenced.Provider.Source)
	if err != nil {
		r.Log.V(1).Info("Skipping Disk.MaxLUN check", "error", err)
		return nil
	}

	plannedByHost := map[string]int{}
	for i := range plan.Spec.VMs {
		vmRef := &plan.Spec.VMs[i].Ref
		if vmRef.NotSet() {
			continue
		}
		v, vErr := inventory.VM(vmRef)
		if vErr != nil {
			if errors.As(vErr, &web.NotFoundError{}) || errors.As(vErr, &web.RefNotUniqueError{}) {
				continue
			}
			r.Log.V(1).Info("Skipping Disk.MaxLUN check for VM", "vm", vmRef.String(), "error", vErr)
			continue
		}
		vm, ok := v.(*vsphere.VM)
		if !ok || vm.Host == "" {
			continue
		}
		for _, disk := range vm.Disks {
			mapping, found := plan.Referenced.Map.Storage.FindStorage(disk.Datastore.ID)
			if found && mapping.OffloadPlugin != nil && mapping.OffloadPlugin.VSphereXcopyPluginConfig != nil {
				plannedByHost[vm.Host]++
			}
		}
	}

	var items []string
	var details []string
	for hostID, planned := range plannedByHost {
		if planned == 0 {
			continue
		}
		host := &vsphere.Host{}
		if fErr := inventory.Find(host, ref.Ref{ID: hostID}); fErr != nil {
			continue
		}
		existing := len(host.HostScsiDisks)
		if existing+planned < defaultDiskMaxLUN {
			continue
		}
		name := hostID
		if host.Name != "" {
			name = host.Name
		}
		items = append(items, hostID)
		details = append(details, fmt.Sprintf("%s (existing≈%d + planned=%d)", name, existing, planned))
	}

	if len(items) == 0 {
		return nil
	}

	plan.Status.SetCondition(libcnd.Condition{
		Type:     DiskMaxLUNTooLow,
		Status:   True,
		Reason:   NotValid,
		Category: Warn,
		Message: fmt.Sprintf(
			"ESXi Disk.MaxLUN may be too low for this copy-offload migration on: %s. "+
				"Increase Disk.MaxLUN to at least 2048 on those hosts (default is 1024).",
			strings.Join(details, ", ")),
		Items: items,
	})
	return nil
}
