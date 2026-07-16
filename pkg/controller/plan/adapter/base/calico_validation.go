package base

import (
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	calicoclient "github.com/kubev2v/forklift/pkg/lib/client/calico"
	"k8s.io/apimachinery/pkg/types"
)

// ResolvedCalicoNAD captures the destination-cluster resources backing a
// Calico-referencing NAD after all resource-level validations have passed.
// Per-VM checks read from this directly instead of re-fetching Network and
// IPPool objects for every VM.
//
// The struct accommodates both Calico network flavours:
//   - l2Bridge (IsVRF false): VLAN is the matched l2Bridge VLAN entry
//     (subnets non-empty) and EligiblePools is the L2Workload-restricted
//     pool set scoped to those subnets.
//   - vrf (IsVRF true): routed L3 — the Network carries no VLANs or
//     subnets, so VLAN is zero-value and EligiblePools is the L3-eligible
//     (enabled, Workload-allowed) pool set.
type ResolvedCalicoNAD struct {
	// Network is the Calico Network CR name referenced by the NAD.
	Network string
	// IsVRF marks the referenced Network as a vrf (routed L3) network.
	IsVRF bool
	// VLAN is the resolved l2Bridge VLAN entry (zero-value when IsVRF).
	VLAN calicoclient.VLANEntry
	// EligiblePools are the IPPools the per-VM IP check runs against:
	// L2Workload pools within the VLAN's subnets for l2Bridge networks,
	// L3-eligible pools for vrf networks.
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
//
// Fields must stay comparable (no slices/maps): the struct is compared
// with == and stays parallel to the per-VM CalicoIssue, which keys a
// dedup map.
type CalicoNADIssue struct {
	NAD     types.NamespacedName
	Kind    CalicoIssueKind
	Network string
	VLAN    uint16
	// RouteTable is the offending kernel route table index for the VRF
	// route-table issue kinds (VRFRouteTableReserved / VRFRouteTableConflict
	// / VRFRouteTablePossibleConflict); zero otherwise.
	RouteTable int64
	// ConflictsWith names the other VRF Network CR sharing RouteTable.
	// Empty on a VRFRouteTableConflict means the index collides with the
	// FelixConfiguration routeTableRanges rather than another Network.
	ConflictsWith string
}

// CalicoValidationResult is the output of ValidateCalicoNADs:
// resource-level issues to report at plan level, warnings that describe
// degraded-but-not-blocked configurations, and a cache of healthy NADs
// for downstream per-VM checks.
type CalicoValidationResult struct {
	Issues   []CalicoNADIssue
	Warnings []CalicoNADIssue
	Cache    *CalicoValidationCache
}

// ResolvedCalicoPrimary captures the destination-cluster resources backing a
// calico-flagged NetworkMap entry (a type: pod destination carrying the
// calico field) after plan-level validation has passed. Per-VM checks read
// from this directly instead of re-fetching Network and IPPool objects for
// every VM.
//
// The struct accommodates both Calico-primary cases:
//   - Case A (calico.network == ""): implicit L3 IPAM. Network/VLAN are zero;
//     L3EligiblePools is the pool set the per-VM check uses to validate IP fit.
//   - Case C (calico.network != ""): L2 attach via named Calico Network CR.
//     Network/VLAN are populated; L2EligiblePools is the L2Workload-restricted
//     pool set whose CIDR is contained in the matched VLAN's subnet(s).
type ResolvedCalicoPrimary struct {
	// Network is the named Calico Network CR (empty for Case A).
	Network string
	// VLAN is the resolved l2Bridge VLAN entry (zero-value for Case A).
	VLAN calicoclient.VLANEntry
	// L2EligiblePools is the L2Workload-restricted pool set for Case C.
	L2EligiblePools []calicoclient.IPPool
	// L3EligiblePools is the L3-eligible pool set for Case A.
	L3EligiblePools []calicoclient.IPPool
	// Source is the NetworkMap entry's source ref — used by per-VM dispatch
	// to identify which NIC source maps to the calico primary entry.
	Source ref.Ref
}

// CalicoPrimaryValidationCache holds resolved state for the (at most one)
// calico-flagged NetworkMap entry that passed plan-level validation. Primary
// is nil when no calico-flagged entry exists OR when plan-level validation
// failed for the entry — per-VM checks treat nil as "skip silently, the
// failure is already surfaced at plan level".
type CalicoPrimaryValidationCache struct {
	Primary *ResolvedCalicoPrimary
}

// CalicoPrimaryIssue is a plan- or VM-level failure for a calico-flagged
// NetworkMap entry. VMRef is zero-value for plan-level issues and populated
// for per-VM issues.
//
// All fields are comparable types, so the struct can be used directly as a
// map key for dedup. Per-VM dispatch uses CalicoPrimaryIssue as the dedup key
// — each per-VM invocation only sees one VMRef, so dedup-within-VM is the
// natural behaviour.
type CalicoPrimaryIssue struct {
	VMRef   ref.Ref
	Kind    CalicoIssueKind
	Network string
	VLAN    uint16
	IP      string
}

// CalicoPrimaryValidationResult is the output of ValidateCalicoPrimary:
// plan-level issues to report at plan level, warnings that describe
// degraded-but-not-blocked configurations, and a cache for downstream
// per-VM checks.
type CalicoPrimaryValidationResult struct {
	Issues   []CalicoPrimaryIssue
	Warnings []CalicoPrimaryIssue
	Cache    *CalicoPrimaryValidationCache
}
