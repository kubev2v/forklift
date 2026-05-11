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

// IPPool is a parsed view of projectcalico.org/v3 IPPool.
type IPPool struct {
	Name string
	CIDR string
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
		pools = append(pools, IPPool{Name: u.GetName(), CIDR: cidr})
	}
	return pools, nil
}

// HasEligiblePool reports whether at least one pool's CIDR is contained
// within at least one of vlanSubnets. When false, Calico IPAM has nothing
// to allocate from for this VLAN and CNI ADD will fail regardless of any
// per-pod IP request.
func HasEligiblePool(pools []IPPool, vlanSubnets []string) bool {
	for i := range pools {
		if poolContainedInAnyVLANSubnet(pools[i].CIDR, vlanSubnets) {
			return true
		}
	}
	return false
}

// EligiblePools returns the subset of pools whose CIDR is contained within
// at least one vlanSubnet. Callers cache the result so per-IP membership
// checks don't repeat the containment filter.
func EligiblePools(pools []IPPool, vlanSubnets []string) []IPPool {
	out := make([]IPPool, 0, len(pools))
	for i := range pools {
		if poolContainedInAnyVLANSubnet(pools[i].CIDR, vlanSubnets) {
			out = append(out, pools[i])
		}
	}
	return out
}

// EligiblePoolForIP returns the first pool that (a) contains the given IP and
// (b) is itself contained within at least one VLAN subnet. Returns nil when
// no pool qualifies.
func EligiblePoolForIP(pools []IPPool, ip string, vlanSubnets []string) *IPPool {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return nil
	}
	for i := range pools {
		p := &pools[i]
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

func ipInCIDR(ip net.IP, cidr string) bool {
	_, n, err := net.ParseCIDR(cidr)
	if err != nil {
		return false
	}
	return n.Contains(ip)
}

// poolContainedInAnyVLANSubnet reports whether the pool CIDR is fully
// contained within one of the VLAN subnets — i.e., every address in the pool
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
