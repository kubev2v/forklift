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
	HostdTmpMemoryLow = "HostdTmpMemoryLow"

	// Default ESXi hostd-tmp memory limit. Concurrent vmkfstools can exceed this;
	// see cmd/vsphere-copy-offload-populator/vmkfstools-wrapper/tweak-esxi-mem.sh.
	defaultHostdTmpMB = 500

	// Planned XCOPY disks per host at/above this count risks hostd-tmp pressure.
	hostdTmpDiskWarnThreshold = 10
)

// validateHostdTmpMemory sets a non-blocking warning when a VSphere XCOPY plan
// has enough disks per ESXi host that concurrent vmkfstools may exceed the
// default hostd-tmp memory limit (~500MB).
func (r *Reconciler) validateHostdTmpMemory(plan *api.Plan) error {
	plan.Status.DeleteCondition(HostdTmpMemoryLow)

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
		r.Log.V(1).Info("Skipping hostd-tmp check", "error", err)
		return nil
	}

	diskCountByHost := map[string]int{}
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
			r.Log.V(1).Info("Skipping hostd-tmp check for VM", "vm", vmRef.String(), "error", vErr)
			continue
		}
		vm, ok := v.(*vsphere.VM)
		if !ok || vm.Host == "" {
			continue
		}
		for _, disk := range vm.Disks {
			mapping, found := plan.Referenced.Map.Storage.FindStorage(disk.Datastore.ID)
			if found && mapping.OffloadPlugin != nil && mapping.OffloadPlugin.VSphereXcopyPluginConfig != nil {
				diskCountByHost[vm.Host]++
			}
		}
	}

	var items []string
	var details []string
	for hostID, planned := range diskCountByHost {
		if planned < hostdTmpDiskWarnThreshold {
			continue
		}
		name := hostID
		host := &vsphere.Host{}
		if fErr := inventory.Find(host, ref.Ref{ID: hostID}); fErr == nil && host.Name != "" {
			name = host.Name
		}
		items = append(items, hostID)
		details = append(details, fmt.Sprintf("%s (%d XCOPY disks)", name, planned))
	}

	if len(items) == 0 {
		return nil
	}

	plan.Status.SetCondition(libcnd.Condition{
		Type:     HostdTmpMemoryLow,
		Status:   True,
		Reason:   NotValid,
		Category: Warn,
		Message: fmt.Sprintf(
			"Concurrent vmkfstools for this copy-offload migration may exceed the default ESXi hostd-tmp memory limit (~%dMB) on: %s. "+
				"Raise hostd-tmp (e.g. run vmkfstools-wrapper/tweak-esxi-mem.sh 2048 on those hosts) before migrating.",
			defaultHostdTmpMB, strings.Join(details, ", ")),
		Items: items,
	})
	return nil
}
