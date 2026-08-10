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

	// Default ESXi hostd-tmp limit (~500MB). See tweak-esxi-mem.sh in the populator.
	defaultHostdTmpMB         = 500
	hostdTmpDiskWarnThreshold = 10
)

func shouldWarnHostdTmp(plannedDisks int) bool {
	return plannedDisks >= hostdTmpDiskWarnThreshold
}

// validateHostdTmpMemory warns when an XCOPY plan may push concurrent vmkfstools
// past the default ESXi hostd-tmp memory limit (~500MB).
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
		if !ok || vm.Host == "" {
			continue
		}
		for _, disk := range vm.Disks {
			m, found := plan.Map.Storage.FindStorage(disk.Datastore.ID)
			if found && m.OffloadPlugin != nil && m.OffloadPlugin.VSphereXcopyPluginConfig != nil {
				disksByHost[vm.Host]++
			}
		}
	}

	var hosts []string
	for hostID, n := range disksByHost {
		if shouldWarnHostdTmp(n) {
			hosts = append(hosts, fmt.Sprintf("%s (%d disks)", hostID, n))
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
			"Concurrent vmkfstools may exceed ESXi hostd-tmp (~%dMB) on: %s. "+
				"Raise hostd-tmp (e.g. tweak-esxi-mem.sh 2048) before migrating.",
			defaultHostdTmpMB, strings.Join(hosts, ", ")),
		Items: hosts,
	})
	return nil
}
