package hyperv

import (
	"testing"

	hyperv "github.com/kubev2v/forklift/pkg/controller/provider/model/hyperv"
)

func TestBuildNICKeys(t *testing.T) {
	tests := []struct {
		name                  string
		nics                  []hyperv.NIC
		vlanQualifiedNetworks map[string]bool
		expected              []string
	}{
		{
			name: "single NIC per network, no VLAN disambiguation",
			nics: []hyperv.NIC{
				{Network: hyperv.Ref{ID: "net-a"}, VlanId: 100},
				{Network: hyperv.Ref{ID: "net-b"}, VlanId: 200},
			},
			vlanQualifiedNetworks: map[string]bool{"net-a": true},
			expected:              []string{"net-a", "net-b"},
		},
		{
			name: "multiple NICs same network with different VLANs and VLAN-qualified map",
			nics: []hyperv.NIC{
				{Network: hyperv.Ref{ID: "net-a"}, VlanId: 100},
				{Network: hyperv.Ref{ID: "net-a"}, VlanId: 200},
			},
			vlanQualifiedNetworks: map[string]bool{"net-a": true},
			expected:              []string{"net-a/100", "net-a/200"},
		},
		{
			name: "multiple NICs same network but NO VLAN-qualified map (backward compat)",
			nics: []hyperv.NIC{
				{Network: hyperv.Ref{ID: "net-a"}, VlanId: 100},
				{Network: hyperv.Ref{ID: "net-a"}, VlanId: 200},
			},
			vlanQualifiedNetworks: map[string]bool{},
			expected:              []string{"net-a", "net-a"},
		},
		{
			name: "multiple NICs same network, one untagged (VlanId=0)",
			nics: []hyperv.NIC{
				{Network: hyperv.Ref{ID: "net-a"}, VlanId: 100},
				{Network: hyperv.Ref{ID: "net-a"}, VlanId: 0},
			},
			vlanQualifiedNetworks: map[string]bool{"net-a": true},
			expected:              []string{"net-a/100", "net-a"},
		},
		{
			name: "single NIC no VLAN",
			nics: []hyperv.NIC{
				{Network: hyperv.Ref{ID: "net-a"}, VlanId: 0},
			},
			vlanQualifiedNetworks: map[string]bool{},
			expected:              []string{"net-a"},
		},
		{
			name: "mixed: one network shared with VLAN map, another unique",
			nics: []hyperv.NIC{
				{Network: hyperv.Ref{ID: "net-a"}, VlanId: 100},
				{Network: hyperv.Ref{ID: "net-a"}, VlanId: 200},
				{Network: hyperv.Ref{ID: "net-b"}, VlanId: 50},
			},
			vlanQualifiedNetworks: map[string]bool{"net-a": true},
			expected:              []string{"net-a/100", "net-a/200", "net-b"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			count := nicNetworkCount(tc.nics)
			keys := buildNICKeys(tc.nics, count, tc.vlanQualifiedNetworks)

			if len(keys) != len(tc.expected) {
				t.Fatalf("expected %d keys, got %d", len(tc.expected), len(keys))
			}
			for i, key := range keys {
				if key != tc.expected[i] {
					t.Errorf("keys[%d] = %q, want %q", i, key, tc.expected[i])
				}
			}
		})
	}
}

func TestBuildPairKey(t *testing.T) {
	tests := []struct {
		name         string
		networkID    string
		vlan         string
		networkCount map[string]int
		expected     string
	}{
		{
			name:         "no VLAN set",
			networkID:    "net-a",
			vlan:         "",
			networkCount: map[string]int{"net-a": 2},
			expected:     "net-a",
		},
		{
			name:         "VLAN set but only one NIC on network (no disambiguation needed)",
			networkID:    "net-a",
			vlan:         "100",
			networkCount: map[string]int{"net-a": 1},
			expected:     "net-a",
		},
		{
			name:         "VLAN set and multiple NICs on network",
			networkID:    "net-a",
			vlan:         "100",
			networkCount: map[string]int{"net-a": 2},
			expected:     "net-a/100",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			key := buildPairKey(tc.networkID, tc.vlan, tc.networkCount)
			if key != tc.expected {
				t.Errorf("buildPairKey(%q, %q, ...) = %q, want %q",
					tc.networkID, tc.vlan, key, tc.expected)
			}
		})
	}
}
