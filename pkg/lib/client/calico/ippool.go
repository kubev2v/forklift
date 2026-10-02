package calico

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

// ParseIPPool parses one projectcalico.org/v3 IPPool object.
func ParseIPPool(u *unstructured.Unstructured) (*IPPool, error) {
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
	return &IPPool{
		Name:        u.GetName(),
		CIDR:        cidr,
		Disabled:    disabled,
		AllowedUses: allowedUses,
	}, nil
}
