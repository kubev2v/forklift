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

// Types
const (
	SSHReady         = "SSHReady"
	SSHNotReady      = "SSHNotReady"
	DiskMaxLUNTooLow = "DiskMaxLUNTooLow"

	// Default ESXi Disk.MaxLUN (LUN IDs 0-1023). Copy-offload README recommends ≥2048.
	defaultDiskMaxLUN = 1024
)

// validateSSHReadiness validates SSH readiness for migration plans using xcopy volume populators
func (r *Reconciler) validateSSHReadiness(plan *api.Plan) error {
	// Check source provider for SSH readiness issues
	sourceProvider := plan.Referenced.Provider.Source
	if sourceProvider == nil {
		return nil // This would be caught by other validation
	}

	// Only check vSphere providers with SSH method enabled
	if sourceProvider.Type() != api.VSphere {
		return nil
	}

	// Only validate SSH readiness for plans using VSphere xcopy volume populators
	if !r.planUsesVSphereXcopyPopulator(plan) {
		// Remove any existing SSH readiness conditions since xcopy is not used
		plan.Status.DeleteCondition(SSHReady)
		plan.Status.DeleteCondition(SSHNotReady)
		return nil
	}

	// Only check when ESXiCloneMethod is set to "ssh"
	esxiCloneMethod, methodSet := sourceProvider.Spec.Settings[api.ESXiCloneMethod]
	if !methodSet || esxiCloneMethod != api.ESXiCloneMethodSSH {
		// Remove any existing SSH readiness conditions since SSH method is not enabled
		plan.Status.DeleteCondition(SSHReady)
		plan.Status.DeleteCondition(SSHNotReady)
		return nil
	}

	// Check for provider SSH ready condition (advisory - hosts that passed validation)
	sshReadyCondition := sourceProvider.Status.FindCondition(SSHReady)
	if sshReadyCondition != nil && sshReadyCondition.Status == libcnd.True && len(sshReadyCondition.Items) > 0 {
		suggestion := fmt.Sprintf("Provider '%s' has ESXi hosts with SSH connectivity validated. ", sourceProvider.Name)
		suggestion += sshReadyCondition.Suggestion

		plan.Status.SetCondition(libcnd.Condition{
			Type:       SSHReady,
			Status:     libcnd.True,
			Reason:     "ProviderSSHReady",
			Category:   libcnd.Advisory,
			Message:    "SSH connectivity validated (checked because 'esxiCloneMethod' setting is set to 'ssh' on the source provider). See the suggestion field in the Plan's YAML for the list of available ESXi hosts.",
			Suggestion: suggestion,
			Items:      formatHostItems(sshReadyCondition.Items),
		})
	} else {
		plan.Status.DeleteCondition(SSHReady)
	}

	// Check for provider SSH not ready condition (warning - hosts that failed validation)
	sshNotReadyCondition := sourceProvider.Status.FindCondition(SSHNotReady)
	if sshNotReadyCondition != nil && sshNotReadyCondition.Status == libcnd.True && len(sshNotReadyCondition.Items) > 0 {
		suggestion := fmt.Sprintf("Migration plan uses xcopy volume populator with provider '%s' that has SSH connectivity issues. ", sourceProvider.Name)
		suggestion += sshNotReadyCondition.Suggestion

		plan.Status.SetCondition(libcnd.Condition{
			Type:       SSHNotReady,
			Status:     libcnd.True,
			Reason:     "ProviderSSHNotReady",
			Category:   libcnd.Warn,
			Message:    "SSH readiness validation issue (checked because 'esxiCloneMethod' setting is set to 'ssh' on the source provider). See the suggestion field in the Plan's YAML for details.",
			Suggestion: suggestion,
			Items:      formatHostItems(sshNotReadyCondition.Items),
		})
	} else {
		plan.Status.DeleteCondition(SSHNotReady)
	}

	return nil
}

// validateDiskMaxLUN sets a non-blocking warning when a VSphere XCOPY plan may
// exceed the default ESXi Disk.MaxLUN limit (existing SCSI disks + planned disks).
func (r *Reconciler) validateDiskMaxLUN(plan *api.Plan) error {
	plan.Status.DeleteCondition(DiskMaxLUNTooLow)

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
			mapping, found := plan.Map.Storage.FindStorage(disk.Datastore.ID)
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

// planUsesVSphereXcopyPopulator checks if a plan uses VSphere xcopy volume populators
func (r *Reconciler) planUsesVSphereXcopyPopulator(plan *api.Plan) bool {
	// Check storage mappings for VSphereXcopyPluginConfig
	if plan.Referenced.Map.Storage == nil {
		return false
	}
	if plan.Referenced.Map.Storage.Spec.Map == nil {
		return false
	}
	dsMapIn := plan.Referenced.Map.Storage.Spec.Map
	for _, mapping := range dsMapIn {
		if mapping.OffloadPlugin != nil && mapping.OffloadPlugin.VSphereXcopyPluginConfig != nil {
			r.Log.V(2).Info("Plan uses VSphere xcopy volume populator", "plan", plan.Name)
			return true
		}
	}

	r.Log.V(2).Info("Plan does not use VSphere xcopy volume populator", "plan", plan.Name)
	return false
}

// formatHostItems transforms Provider host items from "id|name|ip" format to Plan format "id:host-123 ip:10.0.0.1"
func formatHostItems(providerItems []string) []string {
	result := make([]string, 0, len(providerItems))
	for _, item := range providerItems {
		parts := strings.Split(item, "|")
		if len(parts) == 3 {
			// Format: "id:host-123 ip:10.0.0.1"
			result = append(result, fmt.Sprintf("id:%s ip:%s", parts[0], parts[2]))
		} else {
			// Fallback: keep original
			result = append(result, item)
		}
	}
	return result
}
