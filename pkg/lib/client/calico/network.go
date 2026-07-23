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

// VRFHostEntry is a parsed entry of Network.spec.vrf.hostConfig — the
// per-node-set VRF placement the viability checks inspect. staticRoutes
// is deliberately not modelled; no check reads it.
type VRFHostEntry struct {
	// NodeSelector is the Calico node selector choosing which nodes the
	// entry applies to. Empty means the field was absent or empty — the
	// entry then applies to every node.
	NodeSelector string
	// RouteTableIndex is the kernel route table the VRF owns on matching
	// nodes.
	RouteTableIndex int64
	// HasHostInterfaces reports whether the entry names at least one host
	// interface (hostInterfaces non-empty). Only presence is modelled: the
	// checks care that an off-node path exists, not which interface it is.
	HasHostInterfaces bool
}

// Network is a thin parsed view of projectcalico.org/v3 Network.
//
// spec is a strict one-of: an l2Bridge Network carries VLANs; a vrf Network
// is routed (L3, no VLANs). For a vrf Network, VRFHostConfig carries the
// hostConfig entries the VRF viability checks need; the rest of the VRF
// spec is not modelled.
type Network struct {
	Name          string
	L2Bridge      *L2BridgeSpec  // nil when the Network has no l2Bridge spec
	IsVRF         bool           // true when the Network has a vrf spec
	VRFHostConfig []VRFHostEntry // parsed spec.vrf.hostConfig (IsVRF only)
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

// ListNetworks lists every projectcalico.org/v3 Network CR on the cluster,
// parsed into the same view GetNetwork returns. The VRF viability checks use
// it to scan for route-table collisions across Network CRs.
func ListNetworks(ctx context.Context, c client.Client) ([]Network, error) {
	ul := &unstructured.UnstructuredList{}
	ul.SetGroupVersionKind(NetworkGVK.GroupVersion().WithKind("NetworkList"))
	if err := c.List(ctx, ul); err != nil {
		return nil, err
	}
	networks := make([]Network, 0, len(ul.Items))
	for i := range ul.Items {
		n, err := parseNetwork(&ul.Items[i])
		if err != nil {
			return nil, fmt.Errorf("network %q: %w", ul.Items[i].GetName(), err)
		}
		networks = append(networks, *n)
	}
	return networks, nil
}

func parseNetwork(u *unstructured.Unstructured) (*Network, error) {
	n := &Network{Name: u.GetName()}

	vrfMap, isVRF, err := unstructured.NestedMap(u.Object, "spec", "vrf")
	if err != nil {
		return nil, fmt.Errorf("parse spec.vrf: %w", err)
	}
	n.IsVRF = isVRF
	if isVRF {
		n.VRFHostConfig, err = parseVRFHostConfig(vrfMap)
		if err != nil {
			return nil, err
		}
	}

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

// parseVRFHostConfig parses spec.vrf.hostConfig into the minimal entry view
// the VRF viability checks need: nodeSelector, routeTableIndex and whether
// hostInterfaces names anything. routeTableIndex is required by the API, so
// a missing or non-integer value is a parse error; nodeSelector is optional
// (absent means all nodes). hostInterfaces entries come in two API vintages
// — objects with a name field, or plain strings — so only the list's
// non-emptiness is read; the elements themselves are never interpreted.
func parseVRFHostConfig(vrfMap map[string]interface{}) ([]VRFHostEntry, error) {
	entriesRaw, found, err := unstructured.NestedSlice(vrfMap, "hostConfig")
	if err != nil {
		return nil, fmt.Errorf("parse spec.vrf.hostConfig: %w", err)
	}
	if !found {
		return nil, nil
	}
	entries := make([]VRFHostEntry, 0, len(entriesRaw))
	for i, e := range entriesRaw {
		m, ok := e.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("spec.vrf.hostConfig[%d]: not an object", i)
		}
		selector, _, err := unstructured.NestedString(m, "nodeSelector")
		if err != nil {
			return nil, fmt.Errorf("spec.vrf.hostConfig[%d].nodeSelector: %w", i, err)
		}
		idxRaw, found, err := unstructured.NestedFieldNoCopy(m, "routeTableIndex")
		if err != nil {
			return nil, fmt.Errorf("spec.vrf.hostConfig[%d].routeTableIndex: %w", i, err)
		}
		if !found {
			return nil, fmt.Errorf("spec.vrf.hostConfig[%d].routeTableIndex: missing", i)
		}
		index, ok := asInt64(idxRaw)
		if !ok {
			return nil, fmt.Errorf("spec.vrf.hostConfig[%d].routeTableIndex: not an integer (%v)", i, idxRaw)
		}
		ifaces, _, err := unstructured.NestedSlice(m, "hostInterfaces")
		if err != nil {
			return nil, fmt.Errorf("spec.vrf.hostConfig[%d].hostInterfaces: %w", i, err)
		}
		entries = append(entries, VRFHostEntry{
			NodeSelector:      selector,
			RouteTableIndex:   index,
			HasHostInterfaces: len(ifaces) > 0,
		})
	}
	return entries, nil
}

// asInt64 coerces an unstructured numeric field to int64. JSON decoding
// yields int64 for integers, but some encoders produce float64; any other
// type, and any float with a fractional part, reports false.
func asInt64(v interface{}) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case float64:
		i := int64(n)
		if float64(i) != n {
			return 0, false
		}
		return i, true
	}
	return 0, false
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
