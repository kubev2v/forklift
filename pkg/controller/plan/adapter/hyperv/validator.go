package hyperv

import (
	"strings"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	planbase "github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/hyperv"
	webbase "github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/hyperv"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Validator struct {
	*plancontext.Context
}

// HyperV only supports cold migration
func (r *Validator) WarmMigration() bool {
	return false
}

func (r *Validator) MigrationType() bool {
	switch r.Plan.Spec.Type {
	case api.MigrationCold, "":
		return true
	default:
		return false
	}
}

// HyperV uses single SMB share, validation based on VM concerns
func (r *Validator) StorageMapped(vmRef ref.Ref) (bool, error) {
	vm := &hyperv.VM{}
	err := r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		return false, liberr.Wrap(err, "vm", vmRef.String())
	}

	for _, disk := range vm.Disks {
		if disk.SMBPath == "" {
			return false, nil
		}
	}
	return true, nil
}

// NO-OP
func (r *Validator) DirectStorage(_ ref.Ref) (bool, error) {
	return true, nil
}

func (r *Validator) NetworksMapped(vmRef ref.Ref) (bool, error) {
	vm := &hyperv.VM{}
	err := r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		return false, liberr.Wrap(err, "vm", vmRef.String())
	}

	if r.Context.Map.Network == nil {
		return false, nil
	}

	for _, nic := range vm.NICs {
		if nic.Network.ID == "" {
			continue // Disconnected NIC is OK
		}
		mapped := false
		for _, pair := range r.Context.Map.Network.Spec.Map {
			if pair.Source.ID == nic.Network.ID {
				mapped = true
				break
			}
		}
		if !mapped {
			return false, nil
		}
	}
	return true, nil
}

// MaintenanceMode checks whether the VM's owner node is in a Paused or Down
// state. In standalone mode there are no Host records, so it always returns ok.
func (r *Validator) MaintenanceMode(vmRef ref.Ref) (bool, error) {
	if !r.Source.Provider.IsHyperVCluster() {
		return true, nil
	}

	vm := &hyperv.VM{}
	err := r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		return false, liberr.Wrap(err, "vm", vmRef.String())
	}

	if vm.Host == "" {
		return true, nil
	}

	host := &hyperv.Host{}
	hostRef := ref.Ref{ID: vm.Host}
	err = r.Source.Inventory.Find(host, hostRef)
	if err != nil {
		return false, liberr.Wrap(err, "vm", vmRef.String(), "host", hostRef.String())
	}

	switch host.State {
	case model.NodeStatePaused, model.NodeStateDown:
		return false, nil
	default:
		return true, nil
	}
}

// NICNetworkRefs returns one source-network ref per VM NIC.
func (r *Validator) NICNetworkRefs(vmRef ref.Ref) (refs []ref.Ref, err error) {
	vm := &hyperv.VM{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}
	refs = make([]ref.Ref, 0, len(vm.NICs))
	for _, nic := range vm.NICs {
		refs = append(refs, ref.Ref{ID: nic.Network.ID})
	}
	return
}

func (r *Validator) StaticIPs(vmRef ref.Ref) (bool, error) {
	if !r.Plan.Spec.PreserveStaticIPs {
		return true, nil
	}

	vm := &hyperv.VM{}
	err := r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		return false, liberr.Wrap(err, "vm", vmRef.String())
	}

	// Warn if no guest network data - static IPs cannot be preserved without it.
	if len(vm.GuestNetworks) == 0 {
		return false, nil
	}

	for _, guestNetwork := range vm.GuestNetworks {
		found := false
		for _, nic := range vm.NICs {
			if strings.EqualFold(nic.MAC, guestNetwork.MAC) {
				found = true
				break
			}
		}
		if !found {
			return false, nil
		}
	}
	return true, nil
}

// NO-OP
func (r *Validator) UdnStaticIPs(_ ref.Ref, _ client.Client) (bool, error) {
	return true, nil
}

// NO-OP
func (r *Validator) SharedDisks(_ ref.Ref, _ client.Client) (bool, string, string, error) {
	return true, "", "", nil
}

// NO-OP
func (r *Validator) ChangeTrackingEnabled(_ ref.Ref) (bool, error) {
	return true, nil
}

func (r *Validator) HasSnapshot(vmRef ref.Ref) (bool, string, string, error) {
	vm := &hyperv.VM{}
	err := r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		return false, "", "", liberr.Wrap(err, "vm", vmRef.String())
	}
	if vm.HasCheckpoint {
		return false,
			"VM has pre-existing checkpoints (snapshots) which should be removed before migration",
			"Warning",
			nil
	}
	return true, "", "", nil
}

// NO-OP
func (r *Validator) PowerState(vmRef ref.Ref) (bool, error) {
	return true, nil
}

// NO-OP
func (r *Validator) VMMigrationType(_ ref.Ref) (bool, error) {
	return true, nil
}

func (r *Validator) InvalidDiskSizes(vmRef ref.Ref) ([]string, error) {
	vm := &hyperv.VM{}
	err := r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		return nil, liberr.Wrap(err, "vm", vmRef.String())
	}

	var invalid []string
	for _, disk := range vm.Disks {
		if disk.Capacity <= 0 {
			invalid = append(invalid, disk.ID)
		}
	}
	return invalid, nil
}

func (r *Validator) MacConflicts(vmRef ref.Ref) ([]planbase.MacConflict, error) {
	vm, err := planbase.FindSourceVM[hyperv.VM](r.Source.Inventory, vmRef)
	if err != nil {
		return nil, err
	}

	destinationVMs, err := planbase.GetDestinationVMsFromInventory(r.Destination.Inventory, webbase.Param{
		Key:   webbase.DetailParam,
		Value: "all",
	})
	if err != nil {
		return nil, liberr.Wrap(err, "fetching destination VMs for MAC conflict check")
	}

	var sourceMacs []string
	for _, nic := range vm.NICs {
		sourceMacs = append(sourceMacs, nic.MAC)
	}

	return planbase.CheckMacConflicts(sourceMacs, destinationVMs), nil
}

func (r *Validator) PVCNameTemplate(vmRef ref.Ref, pvcNameTemplate string) (bool, error) {
	if pvcNameTemplate == "" {
		return true, nil
	}

	vm := &hyperv.VM{}
	err := r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		return false, liberr.Wrap(err, "vm", vmRef.String())
	}

	// Validate template produces valid k8s labels for each disk
	for i, disk := range vm.Disks {
		testData := map[string]interface{}{
			"VmName":    vm.Name,
			"DiskIndex": i,
			"DiskId":    disk.ID,
		}
		_, err := planbase.ValidatePVCNameTemplate(pvcNameTemplate, testData)
		if err != nil {
			return false, liberr.Wrap(err, "vm", vmRef.String(), "disk", disk.ID)
		}
	}
	return true, nil
}

// NO-OP
func (r *Validator) GuestToolsInstalled(_ ref.Ref) (bool, error) {
	return true, nil
}

// NO-OP
func (r *Validator) ConsolidationNeeded(_ ref.Ref) (bool, error) {
	return false, nil
}

// ValidateCalicoNADs returns empty results. Non-vSphere providers aren't a
// target for Calico-Network IP/MAC preservation today.
func (r *Validator) ValidateCalicoNADs(_ client.Client) (planbase.CalicoValidationResult, error) {
	return planbase.CalicoValidationResult{}, nil
}

// CalicoVMIssues returns no issues. Non-vSphere providers aren't a target
// for Calico-Network IP/MAC preservation today.
func (r *Validator) CalicoVMIssues(_ ref.Ref, _ *planbase.CalicoValidationCache) ([]planbase.CalicoIssue, error) {
	return nil, nil
}

// ValidateCalicoPrimary scans the NetworkMap. If any calico-flagged entry
// exists, returns a single CalicoIssuePrimaryProviderUnsupported issue —
// the feature is not supported on this provider in this release.
func (r *Validator) ValidateCalicoPrimary(_ client.Client) (planbase.CalicoPrimaryValidationResult, error) {
	if r.Plan.Referenced.Map.Network == nil {
		return planbase.CalicoPrimaryValidationResult{}, nil
	}
	for _, pair := range r.Plan.Referenced.Map.Network.Spec.Map {
		if pair.Destination.Calico != nil {
			return planbase.CalicoPrimaryValidationResult{
				Issues: []planbase.CalicoPrimaryIssue{{Kind: planbase.CalicoIssuePrimaryProviderUnsupported}},
			}, nil
		}
	}
	return planbase.CalicoPrimaryValidationResult{}, nil
}

// CalicoPrimaryIssues returns nil; any calico-flagged entry was already
// rejected at plan level by ValidateCalicoPrimary.
func (r *Validator) CalicoPrimaryIssues(_ ref.Ref, _ *planbase.CalicoPrimaryValidationCache) ([]planbase.CalicoPrimaryIssue, error) {
	return nil, nil
}
