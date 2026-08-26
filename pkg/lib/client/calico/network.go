package calico

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// NetworkGVK is the GroupVersionKind of projectcalico.org/v3 Network.
var NetworkGVK = schema.GroupVersionKind{
	Group:   "projectcalico.org",
	Version: "v3",
	Kind:    "Network",
}

// VLANEntry is a parsed l2Bridge.vlans[] entry. The schema's vlan field is
// a one-of: a single ID (vlan.id) or a contiguous range (vlan.range). A
// single ID is represented with VID == RangeStart == RangeEnd, so Covers
// works uniformly for both forms.
type VLANEntry struct {
	// VID is the single VLAN ID, zero when the entry declares a range.
	VID uint16
	// RangeStart/RangeEnd are the inclusive bounds of the entry's VLAN
	// set; for a single-ID entry both equal VID.
	RangeStart uint16
	RangeEnd   uint16
	Subnets    []string
}

// Covers reports whether the entry's VLAN set includes vid.
func (e *VLANEntry) Covers(vid uint16) bool {
	return vid >= e.RangeStart && vid <= e.RangeEnd
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

// ParseNetwork parses one projectcalico.org/v3 Network object.
func ParseNetwork(u *unstructured.Unstructured) (*Network, error) {
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

	// The vlan field is a schema-enforced one-of: a single id or a
	// {start, end} range. Bounds (1-4094) are the schema's to enforce;
	// the parser serves what the API server accepted.
	vidRaw, found, err := unstructured.NestedFieldNoCopy(m, "vlan", "id")
	if err != nil {
		return entry, fmt.Errorf("vlans[%d].vlan.id: %w", idx, err)
	}
	if found {
		id, ok := asInt64(vidRaw)
		if !ok {
			return entry, fmt.Errorf("vlans[%d].vlan.id: not an integer (%v)", idx, vidRaw)
		}
		entry.VID = uint16(id)
		entry.RangeStart = entry.VID
		entry.RangeEnd = entry.VID
	} else {
		startRaw, startFound, err := unstructured.NestedFieldNoCopy(m, "vlan", "range", "start")
		if err != nil {
			return entry, fmt.Errorf("vlans[%d].vlan.range.start: %w", idx, err)
		}
		endRaw, endFound, err := unstructured.NestedFieldNoCopy(m, "vlan", "range", "end")
		if err != nil {
			return entry, fmt.Errorf("vlans[%d].vlan.range.end: %w", idx, err)
		}
		if !startFound || !endFound {
			return entry, fmt.Errorf("vlans[%d].vlan: neither id nor a complete range present", idx)
		}
		start, ok := asInt64(startRaw)
		if !ok {
			return entry, fmt.Errorf("vlans[%d].vlan.range.start: not an integer (%v)", idx, startRaw)
		}
		end, ok := asInt64(endRaw)
		if !ok {
			return entry, fmt.Errorf("vlans[%d].vlan.range.end: not an integer (%v)", idx, endRaw)
		}
		entry.RangeStart = uint16(start)
		entry.RangeEnd = uint16(end)
	}

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
