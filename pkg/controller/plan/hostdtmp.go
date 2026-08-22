package plan

import (
	"errors"
	"fmt"
	"strings"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/provider/web"
	vsphere "github.com/kubev2v/forklift/pkg/controller/provider/web/vsphere"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	"github.com/kubev2v/forklift/pkg/settings"
)

const (
	HostdTmpMemoryLow = "HostdTmpMemoryLow"
)

func shouldWarnHostdTmp(plannedDisks, maxInFlight int) bool {
	if maxInFlight <= 0 {
		return false
	}
	return plannedDisks >= maxInFlight
}

// xcopyRuntimeHosts returns ESXi host IDs where vmkfstools for an XCOPY disk may run.
// Dedicated migration hosts from the storage map take precedence over the VM host.
func xcopyRuntimeHosts(vm *vsphere.VM, config *api.VSphereXcopyPluginConfig) []string {
	if config != nil && len(config.DedicatedMigrationHosts) > 0 {
		return config.DedicatedMigrationHosts
	}
	if vm.Host != "" {
		return []string{vm.Host}
	}
	return nil
}

// validateHostdTmpMemory warns when an XCOPY plan may push concurrent vmkfstools
// past the ESXi hostd-tmp memory limit on a migration host.
func (r *Reconciler) validateHostdTmpMemory(plan *api.Plan) error {
	plan.Status.DeleteCondition(HostdTmpMemoryLow)

	if plan.Provider.Source == nil || plan.Provider.Source.Type() != api.VSphere {
		return nil
	}
	if !settings.Settings.CopyOffload || !r.planUsesVSphereXcopyPopulator(plan) {
		return nil
	}
	if plan.Map.Storage == nil {
		return nil
	}

	inventory, err := web.NewClient(plan.Provider.Source)
	if err != nil {
		r.Log.V(1).Info("Skipping hostd-tmp check", "error", err)
		return nil
	}

	maxInFlight := settings.Settings.MaxInFlight
	disksByHost := map[string]int{}
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
			continue
		}
		vm, ok := v.(*vsphere.VM)
		if !ok {
			continue
		}
		for _, disk := range vm.Disks {
			m, found := plan.Map.Storage.FindStorage(disk.Datastore.ID)
			if !found || m.OffloadPlugin == nil || m.OffloadPlugin.VSphereXcopyPluginConfig == nil {
				continue
			}
			for _, hostID := range xcopyRuntimeHosts(vm, m.OffloadPlugin.VSphereXcopyPluginConfig) {
				disksByHost[hostID]++
			}
		}
	}

	var hosts []string
	var items []string
	for hostID, n := range disksByHost {
		if shouldWarnHostdTmp(n, maxInFlight) {
			hosts = append(hosts, fmt.Sprintf("%s (%d disks)", hostID, n))
			items = append(items, hostID)
		}
	}
	if len(hosts) == 0 {
		return nil
	}

	plan.Status.SetCondition(libcnd.Condition{
		Type:     HostdTmpMemoryLow,
		Status:   True,
		Reason:   NotValid,
		Category: Warn,
		Message: fmt.Sprintf(
			"Concurrent vmkfstools may exceed ESXi hostd-tmp memory on: %s. "+
				"Raise hostd-tmp if needed (see tweak-esxi-mem.sh in vsphere-copy-offload-populator).",
			strings.Join(hosts, ", ")),
		Items: items,
	})
	return nil
}
