package ocp

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestNetworkConfig_UnmarshalCalico(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		wantType      NadType
		wantNetwork   string
		wantVLAN      uint16
		wantIPv4Pools []string
	}{
		{
			name: "CalicoL2WithExplicitVLAN",
			input: `{
				"cniVersion": "0.3.1",
				"type": "calico",
				"network": "datacenter-vlans",
				"vlan": 100,
				"ipam": {"type": "calico-ipam"},
				"datastore_type": "kubernetes",
				"kubernetes": {"kubeconfig": "/etc/cni/net.d/calico-kubeconfig"}
			}`,
			wantType:    CalicoCNIType,
			wantNetwork: "datacenter-vlans",
			wantVLAN:    100,
		},
		{
			name:        "CalicoL2ExplicitVLANZero",
			input:       `{"type":"calico","network":"flat-net","vlan":0}`,
			wantType:    CalicoCNIType,
			wantNetwork: "flat-net",
			wantVLAN:    0,
		},
		{
			name:        "CalicoL3NoNetworkField",
			input:       `{"type":"calico","ipam":{"type":"calico-ipam"}}`,
			wantType:    CalicoCNIType,
			wantNetwork: "",
			wantVLAN:    0,
		},
		{
			name: "OvnKConfigIgnoresCalicoFields",
			input: `{
				"cniVersion": "0.3.1",
				"type": "ovn-k8s-cni-overlay",
				"name": "udn",
				"role": "primary",
				"topology": "layer3",
				"subnets": "10.0.0.0/24"
			}`,
			wantType:    OvnOverlayType,
			wantNetwork: "",
			wantVLAN:    0,
		},
		{
			name:        "UnknownCNI",
			input:       `{"type":"bridge","bridge":"cni0","ipam":{"type":"host-local"}}`,
			wantType:    "bridge",
			wantNetwork: "",
			wantVLAN:    0,
		},
		{
			// ipam.ipv4_pools pins address assignment to specific pools;
			// both pool names and CIDRs are legal entries.
			name:          "CalicoIPAMWithPinnedPools",
			input:         `{"type":"calico","network":"vrf-red","ipam":{"type":"calico-ipam","ipv4_pools":["vrf-red-pool","10.66.0.0/24"]}}`,
			wantType:      CalicoCNIType,
			wantNetwork:   "vrf-red",
			wantIPv4Pools: []string{"vrf-red-pool", "10.66.0.0/24"},
		},
		{
			// ipam block present, ipv4_pools absent → nil (no pin).
			name:        "CalicoIPAMWithoutPinnedPools",
			input:       `{"type":"calico","network":"vrf-red","ipam":{"type":"calico-ipam"}}`,
			wantType:    CalicoCNIType,
			wantNetwork: "vrf-red",
		},
		{
			// An explicitly empty list still means "no pin"; callers key off
			// len(IPv4Pools) == 0.
			name:          "CalicoIPAMEmptyPinnedPools",
			input:         `{"type":"calico","network":"vrf-red","ipam":{"type":"calico-ipam","ipv4_pools":[]}}`,
			wantType:      CalicoCNIType,
			wantNetwork:   "vrf-red",
			wantIPv4Pools: []string{},
		},
		{
			// A non-calico IPAM has no ipv4_pools convention → nil.
			name:        "CalicoWithNonCalicoIPAM",
			input:       `{"type":"calico","network":"vrf-red","ipam":{"type":"dhcp"}}`,
			wantType:    CalicoCNIType,
			wantNetwork: "vrf-red",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NetworkConfig{}
			if err := json.Unmarshal([]byte(tt.input), &cfg); err != nil {
				t.Fatalf("Unmarshal returned unexpected error: %v", err)
			}
			if cfg.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", cfg.Type, tt.wantType)
			}
			if cfg.Network != tt.wantNetwork {
				t.Errorf("Network = %q, want %q", cfg.Network, tt.wantNetwork)
			}
			if cfg.VLAN != tt.wantVLAN {
				t.Errorf("VLAN = %d, want %d", cfg.VLAN, tt.wantVLAN)
			}
			if !reflect.DeepEqual(cfg.IPv4Pools, tt.wantIPv4Pools) {
				t.Errorf("IPv4Pools = %#v, want %#v", cfg.IPv4Pools, tt.wantIPv4Pools)
			}
		})
	}
}

func TestNetworkConfig_ReferencesCalicoNetwork(t *testing.T) {
	tests := []struct {
		name string
		cfg  NetworkConfig
		want bool
	}{
		{
			name: "CalicoTypeAndNetworkSet",
			cfg:  NetworkConfig{Type: CalicoCNIType, Network: "datacenter-vlans"},
			want: true,
		},
		{
			name: "CalicoTypeNoNetwork",
			cfg:  NetworkConfig{Type: CalicoCNIType},
			want: false,
		},
		{
			name: "OvnKTypeWithNetworkField",
			cfg:  NetworkConfig{Type: OvnOverlayType, Network: "datacenter-vlans"},
			want: false,
		},
		{
			name: "EmptyType",
			cfg:  NetworkConfig{Network: "x"},
			want: false,
		},
		{
			name: "Zero",
			cfg:  NetworkConfig{},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.ReferencesCalicoNetwork(); got != tt.want {
				t.Errorf("ReferencesCalicoNetwork() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNetworkConfig_IsUnsupportedUdn_UnaffectedByCalicoFields(t *testing.T) {
	cfg := NetworkConfig{
		Type:    OvnOverlayType,
		Role:    RolePrimary,
		Network: "datacenter-vlans",
		VLAN:    100,
	}
	if !cfg.IsUnsupportedUdn() {
		t.Errorf("IsUnsupportedUdn() = false, want true; Calico fields must not interfere with the UDN predicate")
	}
}
