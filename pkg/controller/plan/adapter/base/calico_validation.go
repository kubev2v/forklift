package base

import (
	calicoclient "github.com/kubev2v/forklift/pkg/lib/client/calico"
	"k8s.io/apimachinery/pkg/types"
)

// ResolvedCalicoNAD captures the destination-cluster resources backing a
// Calico-referencing NAD after all resource-level validations have passed.
// Per-VM checks read from this directly instead of re-fetching Network and
// IPPool objects for every VM.
type ResolvedCalicoNAD struct {
	// Network is the Calico Network CR name referenced by the NAD.
	Network string
	// VLAN is the resolved l2Bridge VLAN entry (subnets non-empty).
	VLAN calicoclient.VLANEntry
	// EligiblePools are the IPPools whose CIDRs overlap any subnet in VLAN.
	EligiblePools []calicoclient.IPPool
}

// CalicoValidationCache holds resolved state for every Calico-referencing
// NAD that passed plan-level validation. NADs with any resource-level issue
// are absent from the map: per-VM checks treat that as "skip silently — the
// failure is already surfaced at plan level".
type CalicoValidationCache struct {
	NADs map[types.NamespacedName]*ResolvedCalicoNAD
}

// CalicoNADIssue is a resource-level Calico failure tied to a specific NAD
// rather than a VM. Surfaced by ValidateCalicoNADs and rendered into the
// plan-level CalicoNetworkInvalid condition.
type CalicoNADIssue struct {
	NAD     types.NamespacedName
	Kind    CalicoIssueKind
	Network string
	VLAN    uint16
}

// CalicoValidationResult is the output of ValidateCalicoNADs:
// resource-level issues to report at plan level, and a cache of healthy
// NADs for downstream per-VM checks.
type CalicoValidationResult struct {
	Issues []CalicoNADIssue
	Cache  *CalicoValidationCache
}
