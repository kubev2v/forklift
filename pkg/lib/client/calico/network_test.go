package calico

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// makeNetwork builds an unstructured projectcalico.org/v3 Network with the
// given name and spec map.
func makeNetwork(name string, spec map[string]interface{}) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(NetworkGVK)
	u.SetName(name)
	if spec != nil {
		_ = unstructured.SetNestedField(u.Object, spec, "spec")
	}
	return u
}

func TestParseNetwork_NoL2Bridge(t *testing.T) {
	// Network exists but has no l2Bridge spec.
	nw := makeNetwork("flat-net", map[string]interface{}{
		// no l2Bridge key
	})
	got, err := ParseNetwork(nw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("got nil network, want non-nil")
	}
	if got.Name != "flat-net" {
		t.Errorf("Name = %q, want flat-net", got.Name)
	}
	if got.L2Bridge != nil {
		t.Errorf("L2Bridge = %+v, want nil", got.L2Bridge)
	}
}

func TestParseNetwork_VRF(t *testing.T) {
	// A VRF (routed, L3) Network: classified via IsVRF, no l2Bridge parsed.
	nw := makeNetwork("routed-net", map[string]interface{}{
		"vrf": map[string]interface{}{
			"hostConfig": []interface{}{
				map[string]interface{}{"nodeSelector": "all()", "routeTableIndex": int64(101)},
			},
		},
	})
	got, err := ParseNetwork(nw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.IsVRF {
		t.Error("IsVRF = false, want true")
	}
	if got.L2Bridge != nil {
		t.Errorf("L2Bridge = %+v, want nil", got.L2Bridge)
	}
	want := []VRFHostEntry{{NodeSelector: "all()", RouteTableIndex: 101}}
	if !reflect.DeepEqual(got.VRFHostConfig, want) {
		t.Errorf("VRFHostConfig = %+v, want %+v", got.VRFHostConfig, want)
	}
}

func TestParseNetwork_VRFHostConfigEntries(t *testing.T) {
	// Multi-entry hostConfig: a scoped entry, an entry with the selector
	// absent (matches all nodes → empty string), and an entry with an
	// explicitly empty selector (same meaning).
	nw := makeNetwork("routed-net", map[string]interface{}{
		"vrf": map[string]interface{}{
			"hostConfig": []interface{}{
				map[string]interface{}{"nodeSelector": "rack == 'a'", "routeTableIndex": int64(101)},
				map[string]interface{}{"routeTableIndex": int64(102)},
				map[string]interface{}{"nodeSelector": "", "routeTableIndex": int64(103)},
			},
		},
	})
	got, err := ParseNetwork(nw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []VRFHostEntry{
		{NodeSelector: "rack == 'a'", RouteTableIndex: 101},
		{NodeSelector: "", RouteTableIndex: 102},
		{NodeSelector: "", RouteTableIndex: 103},
	}
	if !reflect.DeepEqual(got.VRFHostConfig, want) {
		t.Errorf("VRFHostConfig = %+v, want %+v", got.VRFHostConfig, want)
	}
}

func TestParseNetwork_VRFHostInterfaces(t *testing.T) {
	// hostInterfaces comes in two API vintages — objects with a name field,
	// or plain strings. Any non-empty list counts; empty or absent does not.
	nw := makeNetwork("routed-net", map[string]interface{}{
		"vrf": map[string]interface{}{
			"hostConfig": []interface{}{
				map[string]interface{}{
					"routeTableIndex": int64(101),
					"hostInterfaces":  []interface{}{map[string]interface{}{"name": "eth1"}},
				},
				map[string]interface{}{
					"routeTableIndex": int64(102),
					"hostInterfaces":  []interface{}{"eth1", "eth2"},
				},
				map[string]interface{}{
					"routeTableIndex": int64(103),
					"hostInterfaces":  []interface{}{},
				},
				map[string]interface{}{
					"routeTableIndex": int64(104),
				},
			},
		},
	})
	got, err := ParseNetwork(nw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []VRFHostEntry{
		{RouteTableIndex: 101, HasHostInterfaces: true},
		{RouteTableIndex: 102, HasHostInterfaces: true},
		{RouteTableIndex: 103},
		{RouteTableIndex: 104},
	}
	if !reflect.DeepEqual(got.VRFHostConfig, want) {
		t.Errorf("VRFHostConfig = %+v, want %+v", got.VRFHostConfig, want)
	}
}

func TestParseNetwork_VRFBadHostConfig(t *testing.T) {
	cases := []struct {
		name  string
		entry map[string]interface{}
	}{
		{
			name:  "missing routeTableIndex",
			entry: map[string]interface{}{"nodeSelector": "all()"},
		},
		{
			name:  "non-integer routeTableIndex",
			entry: map[string]interface{}{"routeTableIndex": 1.5},
		},
		{
			name:  "string routeTableIndex",
			entry: map[string]interface{}{"routeTableIndex": "101"},
		},
		{
			name: "hostInterfaces not a list",
			entry: map[string]interface{}{
				"routeTableIndex": int64(101),
				"hostInterfaces":  "eth1",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			nw := makeNetwork("bad-vrf", map[string]interface{}{
				"vrf": map[string]interface{}{
					"hostConfig": []interface{}{tc.entry},
				},
			})
			if _, err := ParseNetwork(nw); err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestParseNetwork_Flavours(t *testing.T) {
	l2 := makeNetwork("vlan-net", map[string]interface{}{
		"l2Bridge": map[string]interface{}{
			"vlans": []interface{}{
				map[string]interface{}{
					"vlan":    map[string]interface{}{"id": int64(100)},
					"subnets": []interface{}{map[string]interface{}{"cidr": "10.100.0.0/24"}},
				},
			},
		},
	})
	vrfA := makeNetwork("vrf-a", map[string]interface{}{
		"vrf": map[string]interface{}{
			"hostConfig": []interface{}{
				map[string]interface{}{"routeTableIndex": int64(101)},
			},
		},
	})
	vrfB := makeNetwork("vrf-b", map[string]interface{}{
		"vrf": map[string]interface{}{
			"hostConfig": []interface{}{
				map[string]interface{}{"nodeSelector": "rack == 'b'", "routeTableIndex": int64(102)},
			},
		},
	})
	byName := map[string]Network{}
	for _, u := range []*unstructured.Unstructured{l2, vrfA, vrfB} {
		n, err := ParseNetwork(u)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		byName[n.Name] = *n
	}
	if n := byName["vlan-net"]; n.IsVRF || n.L2Bridge == nil || len(n.L2Bridge.VLANs) != 1 {
		t.Errorf("vlan-net parsed as %+v, want one-VLAN l2Bridge network", n)
	}
	if n := byName["vrf-a"]; !n.IsVRF ||
		!reflect.DeepEqual(n.VRFHostConfig, []VRFHostEntry{{RouteTableIndex: 101}}) {
		t.Errorf("vrf-a parsed as %+v, want all-nodes VRF on table 101", n)
	}
	if n := byName["vrf-b"]; !n.IsVRF ||
		!reflect.DeepEqual(n.VRFHostConfig, []VRFHostEntry{{NodeSelector: "rack == 'b'", RouteTableIndex: 102}}) {
		t.Errorf("vrf-b parsed as %+v, want scoped VRF on table 102", n)
	}
}

func TestParseNetwork_SingleVLAN(t *testing.T) {
	nw := makeNetwork("vlan100", map[string]interface{}{
		"l2Bridge": map[string]interface{}{
			"vlans": []interface{}{
				map[string]interface{}{
					"vlan": map[string]interface{}{"id": int64(100)},
					"subnets": []interface{}{
						map[string]interface{}{"cidr": "10.100.0.0/24"},
					},
				},
			},
		},
	})
	got, err := ParseNetwork(nw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.IsVRF {
		t.Error("IsVRF = true, want false")
	}
	if got.L2Bridge == nil {
		t.Fatal("L2Bridge = nil, want non-nil")
	}
	if len(got.L2Bridge.VLANs) != 1 {
		t.Fatalf("VLANs len = %d, want 1", len(got.L2Bridge.VLANs))
	}
	v := got.L2Bridge.VLANs[0]
	if v.VID != 100 {
		t.Errorf("VID = %d, want 100", v.VID)
	}
	if len(v.Subnets) != 1 || v.Subnets[0] != "10.100.0.0/24" {
		t.Errorf("Subnets = %v, want [10.100.0.0/24]", v.Subnets)
	}
}

func TestParseNetwork_MultipleVLANs(t *testing.T) {
	nw := makeNetwork("datacenter-vlans", map[string]interface{}{
		"l2Bridge": map[string]interface{}{
			"vlans": []interface{}{
				map[string]interface{}{
					"vlan":    map[string]interface{}{"id": int64(100)},
					"subnets": []interface{}{map[string]interface{}{"cidr": "10.100.0.0/24"}},
				},
				map[string]interface{}{
					"vlan":    map[string]interface{}{"id": int64(200)},
					"subnets": []interface{}{map[string]interface{}{"cidr": "10.200.0.0/24"}},
				},
				map[string]interface{}{
					"vlan": map[string]interface{}{"id": int64(300)},
					"subnets": []interface{}{
						map[string]interface{}{"cidr": "10.30.0.0/16"},
						map[string]interface{}{"cidr": "10.31.0.0/16"},
					},
				},
			},
		},
	})
	got, err := ParseNetwork(nw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.L2Bridge == nil {
		t.Fatal("L2Bridge = nil, want non-nil")
	}
	if len(got.L2Bridge.VLANs) != 3 {
		t.Fatalf("VLANs len = %d, want 3", len(got.L2Bridge.VLANs))
	}
	wantVIDs := []uint16{100, 200, 300}
	for i, want := range wantVIDs {
		if got.L2Bridge.VLANs[i].VID != want {
			t.Errorf("VLANs[%d].VID = %d, want %d", i, got.L2Bridge.VLANs[i].VID, want)
		}
	}
	// Multi-subnet VLAN entry
	multi := got.L2Bridge.VLANs[2].Subnets
	if len(multi) != 2 || multi[0] != "10.30.0.0/16" || multi[1] != "10.31.0.0/16" {
		t.Errorf("VLANs[2].Subnets = %v, want [10.30.0.0/16 10.31.0.0/16]", multi)
	}
}

func TestParseNetwork_VLANForms(t *testing.T) {
	// The vlan field is a schema-enforced one-of: a single id, or a
	// {start, end} range. Value bounds are the schema's to enforce; the
	// parser must represent both forms and reject only structural
	// nonsense (neither form present).
	nw := makeNetwork("forms", map[string]interface{}{
		"l2Bridge": map[string]interface{}{
			"vlans": []interface{}{
				map[string]interface{}{
					"vlan":    map[string]interface{}{"id": int64(100)},
					"subnets": []interface{}{map[string]interface{}{"cidr": "10.100.0.0/24"}},
				},
				map[string]interface{}{
					"vlan": map[string]interface{}{
						"range": map[string]interface{}{"start": int64(200), "end": int64(210)},
					},
					"subnets": []interface{}{map[string]interface{}{"cidr": "10.200.0.0/22"}},
				},
			},
		},
	})
	got, err := ParseNetwork(nw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	vlans := got.L2Bridge.VLANs
	if len(vlans) != 2 {
		t.Fatalf("len(vlans) = %d, want 2", len(vlans))
	}
	single := vlans[0]
	if single.VID != 100 || !single.Covers(100) || single.Covers(101) {
		t.Errorf("single-id entry = %+v, want VID 100 covering only 100", single)
	}
	ranged := vlans[1]
	if ranged.VID != 0 || ranged.RangeStart != 200 || ranged.RangeEnd != 210 {
		t.Errorf("range entry = %+v, want 200-210 with zero VID", ranged)
	}
	if !ranged.Covers(200) || !ranged.Covers(205) || !ranged.Covers(210) || ranged.Covers(211) {
		t.Errorf("range entry coverage wrong: %+v", ranged)
	}

	// Structural nonsense: neither id nor a complete range.
	bad := makeNetwork("bad", map[string]interface{}{
		"l2Bridge": map[string]interface{}{
			"vlans": []interface{}{
				map[string]interface{}{"vlan": map[string]interface{}{}},
			},
		},
	})
	if _, err := ParseNetwork(bad); err == nil {
		t.Errorf("expected error for vlan with neither id nor range")
	}
}
