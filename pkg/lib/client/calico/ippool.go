package calico

import (
	"context"
	"fmt"
	"net"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// IPPoolGVK is the GroupVersionKind of projectcalico.org/v3 IPPool.
var IPPoolGVK = schema.GroupVersionKind{
	Group:   "projectcalico.org",
	Version: "v3",
	Kind:    "IPPool",
}

// AllowedUseL2Workload is the value Calico uses in spec.allowedUses to mark an
// IPPool as usable for L2 workloads (i.e. attached via a Calico Network CR's
// l2Bridge VLAN). An IPPool without L2Workload in its allowedUses (including
// the default-when-absent ["Workload","Tunnel"]) is not eligible for L2 attach.
const AllowedUseL2Workload = "L2Workload"

// AllowedUseWorkload is the value Calico uses in spec.allowedUses to mark an
// IPPool as a source of workload (pod) address assignments. Calico IPAM only
// assigns workload addresses from pools that permit this use (explicitly, or
// via the default-when-absent ["Workload","Tunnel"]).
const AllowedUseWorkload = "Workload"

// IPPool is a parsed view of projectcalico.org/v3 IPPool.
//
// AllowedUses distinguishes "absent" (nil) from "explicitly empty" ([]string{}).
// When the spec.allowedUses field is absent in the manifest, Calico applies the
// default ["Workload","Tunnel"] — L3-usable but not L2Workload-usable. The
// helpers in this file treat nil and explicit-empty accordingly.
type IPPool struct {
	Name        string
	CIDR        string
	Disabled    bool
	AllowedUses []string
}

// ListIPPools lists all projectcalico.org/v3 IPPool CRs on the cluster.
func ListIPPools(ctx context.Context, c client.Client) ([]IPPool, error) {
	ul := &unstructured.UnstructuredList{}
	ul.SetGroupVersionKind(IPPoolGVK.GroupVersion().WithKind("IPPoolList"))
	if err := c.List(ctx, ul); err != nil {
		return nil, err
	}
	pools := make([]IPPool, 0, len(ul.Items))
	for i := range ul.Items {
		u := &ul.Items[i]
		cidr, _, err := unstructured.NestedString(u.Object, "spec", "cidr")
		if err != nil {
			return nil, fmt.Errorf("ippool %q: parse spec.cidr: %w", u.GetName(), err)
		}
		disabled, _, err := unstructured.NestedBool(u.Object, "spec", "disabled")
		if err != nil {
			return nil, fmt.Errorf("ippool %q: parse spec.disabled: %w", u.GetName(), err)
		}
		// nil vs explicit-empty distinction is load-bearing for the L3/L2 helpers.
		var allowedUses []string
		rawUses, found, err := unstructured.NestedSlice(u.Object, "spec", "allowedUses")
		if err != nil {
			return nil, fmt.Errorf("ippool %q: parse spec.allowedUses: %w", u.GetName(), err)
		}
		if found {
			allowedUses = make([]string, 0, len(rawUses))
			for _, v := range rawUses {
				s, ok := v.(string)
				if !ok {
					return nil, fmt.Errorf("ippool %q: spec.allowedUses contains non-string entry %T", u.GetName(), v)
				}
				allowedUses = append(allowedUses, s)
			}
		}
		pools = append(pools, IPPool{
			Name:        u.GetName(),
			CIDR:        cidr,
			Disabled:    disabled,
			AllowedUses: allowedUses,
		})
	}
	return pools, nil
}

func ipInCIDR(ip net.IP, cidr string) bool {
	_, n, err := net.ParseCIDR(cidr)
	if err != nil {
		return false
	}
	return n.Contains(ip)
}

// isL3Eligible reports whether a pool can serve as a source of IP allocations
// for a Calico-primary pod without L2 attach (Case A — implicit L3 IPAM).
// A nil AllowedUses means the field was absent and the Calico default
// ["Workload","Tunnel"] applies (workload-assignable). A non-nil slice is
// eligible iff it contains "Workload" — Calico IPAM only assigns workload
// addresses from pools that permit that use, so a Tunnel- or
// LoadBalancer-only pool (and an explicit empty slice) is not eligible.
func isL3Eligible(p *IPPool) bool {
	if p.Disabled {
		return false
	}
	if p.AllowedUses == nil {
		return true
	}
	return containsAllowedUse(p, AllowedUseWorkload)
}

// containsAllowedUse reports whether AllowedUses explicitly lists the named
// use. A nil slice (field absent) lists nothing — callers that must honour
// Calico's default-when-absent check nil before calling (see isL3Eligible).
func containsAllowedUse(p *IPPool, use string) bool {
	for _, u := range p.AllowedUses {
		if u == use {
			return true
		}
	}
	return false
}

// L3EligiblePools returns the subset of pools usable for Case A — implicit
// L3 IPAM — Calico-primary attach. A pool is L3-eligible when it is not
// disabled and its allowedUses contains "Workload" (or is absent, implying
// the Calico default ["Workload","Tunnel"]).
func L3EligiblePools(pools []IPPool) []IPPool {
	out := make([]IPPool, 0, len(pools))
	for i := range pools {
		if isL3Eligible(&pools[i]) {
			out = append(out, pools[i])
		}
	}
	return out
}

// L3EligiblePoolForIP returns the first L3-eligible pool that contains the
// given IP. Returns nil when no pool qualifies. L3 eligibility is intrinsic
// to the pool (no VLAN-subnet containment required).
func L3EligiblePoolForIP(pools []IPPool, ip string) *IPPool {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return nil
	}
	for i := range pools {
		p := &pools[i]
		if !isL3Eligible(p) {
			continue
		}
		if !ipInCIDR(parsedIP, p.CIDR) {
			continue
		}
		return p
	}
	return nil
}

// L2WorkloadEligiblePools returns the subset of pools usable for the L2-attach
// path (attach via a named Network CR). A pool is L2Workload-eligible
// when it is not disabled, its allowedUses contains "L2Workload", and its CIDR
// is fully contained in at least one of the matched VLAN's subnets.
func L2WorkloadEligiblePools(pools []IPPool, vlanSubnets []string) []IPPool {
	out := make([]IPPool, 0, len(pools))
	for i := range pools {
		p := &pools[i]
		if p.Disabled {
			continue
		}
		if !containsAllowedUse(p, AllowedUseL2Workload) {
			continue
		}
		if !poolContainedInAnyVLANSubnet(p.CIDR, vlanSubnets) {
			continue
		}
		out = append(out, pools[i])
	}
	return out
}

// L2WorkloadEligiblePoolForIP returns the first L2Workload-eligible pool that
// contains the given IP. Returns nil when no pool qualifies.
func L2WorkloadEligiblePoolForIP(pools []IPPool, ip string, vlanSubnets []string) *IPPool {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return nil
	}
	for i := range pools {
		p := &pools[i]
		if p.Disabled {
			continue
		}
		if !containsAllowedUse(p, AllowedUseL2Workload) {
			continue
		}
		if !ipInCIDR(parsedIP, p.CIDR) {
			continue
		}
		if !poolContainedInAnyVLANSubnet(p.CIDR, vlanSubnets) {
			continue
		}
		return p
	}
	return nil
}

// poolContainedInAnyVLANSubnet reports whether the pool CIDR is fully
// contained within one of the VLAN subnets, i.e., every address in the pool
// also belongs to the VLAN subnet.
func poolContainedInAnyVLANSubnet(poolCIDR string, vlanSubnets []string) bool {
	_, poolNet, err := net.ParseCIDR(poolCIDR)
	if err != nil {
		return false
	}
	poolMaskBits, _ := poolNet.Mask.Size()
	for _, vs := range vlanSubnets {
		_, vlanNet, err := net.ParseCIDR(vs)
		if err != nil {
			continue
		}
		vlanMaskBits, _ := vlanNet.Mask.Size()
		// pool contained in vlan only if vlan prefix is shorter (or equal)
		// AND the pool's network address belongs to the vlan subnet.
		if vlanMaskBits <= poolMaskBits && vlanNet.Contains(poolNet.IP) {
			return true
		}
	}
	return false
}
