package calico

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NetworkGVK is the GroupVersionKind of projectcalico.org/v3 Network.
var NetworkGVK = schema.GroupVersionKind{
	Group:   "projectcalico.org",
	Version: "v3",
	Kind:    "Network",
}

// VLANEntry is a parsed entry of Network.spec.l2Bridge.vlans.
type VLANEntry struct {
	VID     uint16
	Subnets []string
}

// L2BridgeSpec holds the fields of Network.spec.l2Bridge that the validator
// inspects.
type L2BridgeSpec struct {
	VLANs []VLANEntry
}

// Network is a thin parsed view of projectcalico.org/v3 Network.
type Network struct {
	Name     string
	L2Bridge *L2BridgeSpec // nil when the Network has no l2Bridge spec
}

// GetNetwork fetches projectcalico.org/v3 Network/name from the destination
// cluster and returns a parsed Network. The CR is cluster-scoped.
func GetNetwork(ctx context.Context, c client.Client, name string) (*Network, error) {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(NetworkGVK)
	if err := c.Get(ctx, client.ObjectKey{Name: name}, u); err != nil {
		return nil, err
	}
	return parseNetwork(u)
}

func parseNetwork(u *unstructured.Unstructured) (*Network, error) {
	n := &Network{Name: u.GetName()}

	l2Bridge, found, err := unstructured.NestedMap(u.Object, "spec", "l2Bridge")
	if err != nil {
		return nil, fmt.Errorf("parse spec.l2Bridge: %w", err)
	}
	if !found {
		return n, nil
	}

	vlansRaw, found, err := unstructured.NestedSlice(l2Bridge, "vlans")
	if err != nil {
		return nil, fmt.Errorf("parse spec.l2Bridge.vlans: %w", err)
	}
	spec := &L2BridgeSpec{}
	if found {
		for i, v := range vlansRaw {
			entryMap, ok := v.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("spec.l2Bridge.vlans[%d]: not an object", i)
			}
			entry, err := parseVLANEntry(entryMap, i)
			if err != nil {
				return nil, err
			}
			spec.VLANs = append(spec.VLANs, entry)
		}
	}
	n.L2Bridge = spec
	return n, nil
}

func parseVLANEntry(m map[string]interface{}, idx int) (VLANEntry, error) {
	entry := VLANEntry{}

	vidRaw, found, err := unstructured.NestedFieldNoCopy(m, "vlan", "id")
	if err != nil {
		return entry, fmt.Errorf("vlans[%d].vlan.id: %w", idx, err)
	}
	if !found {
		return entry, fmt.Errorf("vlans[%d].vlan.id: missing", idx)
	}
	var id int64
	switch v := vidRaw.(type) {
	case int64:
		id = v
	case float64:
		id = int64(v)
		if float64(id) != v {
			return entry, fmt.Errorf("vlans[%d].vlan.id: non-integer %g", idx, v)
		}
	default:
		return entry, fmt.Errorf("vlans[%d].vlan.id: unexpected type %T", idx, vidRaw)
	}
	if id < 1 || id > 4094 {
		return entry, fmt.Errorf("vlans[%d].vlan.id: %d out of range (1-4094)", idx, id)
	}
	entry.VID = uint16(id)

	subnetsRaw, _, err := unstructured.NestedSlice(m, "subnets")
	if err != nil {
		return entry, fmt.Errorf("vlans[%d].subnets: %w", idx, err)
	}
	for j, s := range subnetsRaw {
		sMap, ok := s.(map[string]interface{})
		if !ok {
			return entry, fmt.Errorf("vlans[%d].subnets[%d]: not an object", idx, j)
		}
		cidr, _, err := unstructured.NestedString(sMap, "cidr")
		if err != nil {
			return entry, fmt.Errorf("vlans[%d].subnets[%d].cidr: %w", idx, j, err)
		}
		if cidr != "" {
			entry.Subnets = append(entry.Subnets, cidr)
		}
	}
	return entry, nil
}
