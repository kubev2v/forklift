package plan

import (
	"fmt"
	"strings"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/provider/web"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/vsphere"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
)

const (
	VIBReady    = "VIBReady"
	VIBNotReady = "VIBNotReady"
)

func (r *Reconciler) validateVIBReadiness(plan *api.Plan) error {
	if plan.Status.HasCondition(Executing) {
		return nil
	}

	sourceProvider := plan.Referenced.Provider.Source
	if sourceProvider == nil {
		return nil
	}

	if sourceProvider.Type() != api.VSphere {
		return nil
	}

	if !r.planUsesVSphereXcopyPopulator(plan) {
		plan.Status.DeleteCondition(VIBReady)
		plan.Status.DeleteCondition(VIBNotReady)
		return nil
	}

	if !sourceProvider.UseVIBMethod() {
		plan.Status.DeleteCondition(VIBReady)
		plan.Status.DeleteCondition(VIBNotReady)
		return nil
	}

	// Parse Provider's VIBReady Items (format: "id|name") into a set keyed by MoRef ID.
	vibReadyCond := sourceProvider.Status.FindCondition(VIBReady)
	readyHostIDs := map[string]bool{}
	if vibReadyCond != nil {
		for _, item := range vibReadyCond.Items {
			id, _, found := strings.Cut(item, "|")
			if found {
				readyHostIDs[id] = true
			} else {
				r.Log.Info("VIB validation: skipping malformed VIBReady item", "item", item)
				continue
			}
		}
	}

	inventory, err := web.NewClient(sourceProvider)
	if err != nil {
		r.Log.Error(err, "VIB validation: failed to create inventory client", "provider", sourceProvider.Name)
		return err
	}

	// Parse Provider's VIBNotReady Items to get id→name mapping for display.
	vibNotReadyCond := sourceProvider.Status.FindCondition(VIBNotReady)
	hostNames := map[string]string{}
	if vibNotReadyCond != nil {
		for _, item := range vibNotReadyCond.Items {
			id, name, found := strings.Cut(item, "|")
			if found {
				hostNames[id] = name
			} else {
				r.Log.Info("VIB validation: skipping malformed VIBNotReady item", "item", item)
				continue
			}
		}
	}
	if vibReadyCond != nil {
		for _, item := range vibReadyCond.Items {
			id, name, found := strings.Cut(item, "|")
			if found {
				hostNames[id] = name
			} else {
				r.Log.Info("VIB validation: skipping malformed VIBReady item", "item", item)
				continue
			}
		}
	}

	var notReadyHosts []string
	checkedHosts := map[string]bool{}

	for i := range plan.Spec.VMs {
		vm := &plan.Spec.VMs[i]
		v, err := inventory.VM(&vm.Ref)
		if err != nil {
			r.Log.V(2).Info("VIB validation: failed to get VM from inventory, skipping", "vm", vm.Ref.ID, "error", err)
			continue
		}
		vsphereVM, ok := v.(*vsphere.VM)
		if !ok {
			r.Log.V(3).Info("VIB validation: VM is not a vSphere VM, skipping", "vm", vm.Ref.ID)
			continue
		}

		hostID := vsphereVM.Host
		if hostID == "" {
			r.Log.V(2).Info("VM has no host assigned, skipping VIB check", "vm", vm.Ref.ID)
			continue
		}
		if checkedHosts[hostID] {
			continue
		}
		checkedHosts[hostID] = true

		if !readyHostIDs[hostID] {
			name := hostNames[hostID]
			if name == "" {
				name = hostID
			}
			r.Log.V(2).Info("Host not in VIB ready set", "hostID", hostID, "vm", vm.Ref.ID, "readyHosts", len(readyHostIDs))
			notReadyHosts = append(notReadyHosts, fmt.Sprintf("%s|%s", hostID, name))
		}
	}

	if len(notReadyHosts) > 0 {
		plan.Status.SetCondition(libcnd.Condition{
			Type:     VIBNotReady,
			Status:   libcnd.True,
			Reason:   "HostVIBNotInstalled",
			Category: libcnd.Warn,
			Message:  fmt.Sprintf("VIB not installed on %d host(s) required by this plan.", len(notReadyHosts)),
			Items:    notReadyHosts,
		})
		plan.Status.DeleteCondition(VIBReady)
	} else {
		plan.Status.DeleteCondition(VIBNotReady)
		plan.Status.SetCondition(libcnd.Condition{
			Type:     VIBReady,
			Status:   libcnd.True,
			Reason:   "AllHostsReady",
			Category: libcnd.Required,
			Message:  "VIB installed on all hosts required by this plan.",
		})
	}

	return nil
}
