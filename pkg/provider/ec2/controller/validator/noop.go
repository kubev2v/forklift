package validator

import (
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GuestToolsInstalled validates guest tools (not applicable for EC2).
func (r *Validator) GuestToolsInstalled(vmRef ref.Ref) (ok bool, err error) {
	return true, nil
}

// MaintenanceMode validates maintenance mode (not applicable for EC2).
func (r *Validator) MaintenanceMode(vmRef ref.Ref) (ok bool, err error) {
	return true, nil
}

// NICNetworkRefs returns NIC network references (not applicable for EC2).
func (r *Validator) NICNetworkRefs(vmRef ref.Ref) ([]ref.Ref, error) {
	return nil, nil
}

// ChangeTrackingEnabled validates change tracking.
func (r *Validator) ChangeTrackingEnabled(vmRef ref.Ref) (bool, error) {
	return true, nil
}

// DirectStorage validates direct storage access.
func (r *Validator) DirectStorage(vmRef ref.Ref) (bool, error) {
	return true, nil
}

// HasSnapshot validates existing snapshots (not applicable for EC2).
func (r *Validator) HasSnapshot(vmRef ref.Ref) (ok bool, msg string, category string, err error) {
	return true, "", "", nil
}

// InvalidDiskSizes validates disk sizes.
func (r *Validator) InvalidDiskSizes(vmRef ref.Ref) ([]string, error) {
	return nil, nil
}

// MacConflicts validates MAC address conflicts (not applicable for EC2).
func (r *Validator) MacConflicts(vmRef ref.Ref) ([]base.MacConflict, error) {
	return nil, nil
}

// PowerState validates power state requirements.
func (r *Validator) PowerState(vmRef ref.Ref) (ok bool, err error) {
	return true, nil
}

// PVCNameTemplate validates PVC name template (not applicable for EC2).
func (r *Validator) PVCNameTemplate(vmRef ref.Ref, pvcNameTemplate string) (ok bool, err error) {
	return true, nil
}

// SharedDisks validates shared disk configuration (not applicable for EC2).
func (r *Validator) SharedDisks(vmRef ref.Ref, c client.Client) (ok bool, msg string, category string, err error) {
	ok = true
	return
}

// StaticIPs validates static IP configuration.
func (r *Validator) StaticIPs(vmRef ref.Ref) (bool, error) {
	return true, nil
}

// UdnStaticIPs validates UDN static IPs (not applicable for EC2).
func (r *Validator) UdnStaticIPs(vmRef ref.Ref, c client.Client) (ok bool, err error) {
	return true, nil
}

// VMMigrationType validates VM migration type compatibility.
func (r *Validator) VMMigrationType(vmRef ref.Ref) (bool, error) {
	return true, nil
}

// WarmMigration returns false - EC2 only supports cold migration with instance shutdown.
func (r *Validator) WarmMigration() bool {
	return false
}

// ConsolidationNeeded returns false - snapshot consolidation not applicable for EC2.
func (r *Validator) ConsolidationNeeded(vmRef ref.Ref) (bool, error) {
	return false, nil
}

// ValidateCalicoNADs returns empty results (not applicable for EC2).
func (r *Validator) ValidateCalicoNADs(_ client.Client) (base.CalicoValidationResult, error) {
	return base.CalicoValidationResult{}, nil
}

// CalicoVMIssues returns no issues (not applicable for EC2).
func (r *Validator) CalicoVMIssues(_ ref.Ref, _ *base.CalicoValidationCache) ([]base.CalicoIssue, error) {
	return nil, nil
}

// ValidateCalicoPrimary scans the NetworkMap. If any calico-flagged entry
// exists, returns a single CalicoIssuePrimaryProviderUnsupported issue —
// the feature is not supported on this provider in this release.
func (r *Validator) ValidateCalicoPrimary(_ client.Client) (base.CalicoPrimaryValidationResult, error) {
	if r.Plan.Referenced.Map.Network == nil {
		return base.CalicoPrimaryValidationResult{}, nil
	}
	for _, pair := range r.Plan.Referenced.Map.Network.Spec.Map {
		if pair.Destination.Calico != nil {
			return base.CalicoPrimaryValidationResult{
				Issues: []base.CalicoPrimaryIssue{{Kind: base.CalicoIssuePrimaryProviderUnsupported}},
			}, nil
		}
	}
	return base.CalicoPrimaryValidationResult{}, nil
}

// CalicoPrimaryIssues returns nil; any calico-flagged entry was already
// rejected at plan level by ValidateCalicoPrimary.
func (r *Validator) CalicoPrimaryIssues(_ ref.Ref, _ *base.CalicoPrimaryValidationCache) ([]base.CalicoPrimaryIssue, error) {
	return nil, nil
}
