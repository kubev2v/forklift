package calico

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func makeIPPool(name, cidr string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(IPPoolGVK)
	u.SetName(name)
	_ = unstructured.SetNestedField(u.Object, cidr, "spec", "cidr")
	return u
}

func newFakeClientWithIPPools(objs ...runtime.Object) *fake.ClientBuilder {
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(IPPoolGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(IPPoolGVK.GroupVersion().WithKind("IPPoolList"), &unstructured.UnstructuredList{})
	b := fake.NewClientBuilder().WithScheme(scheme)
	for _, o := range objs {
		b = b.WithRuntimeObjects(o)
	}
	return b
}

func TestListIPPools_Empty(t *testing.T) {
	c := newFakeClientWithIPPools().Build()
	pools, err := ListIPPools(context.Background(), c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pools) != 0 {
		t.Errorf("got %d pools, want 0", len(pools))
	}
}

func TestListIPPools_Multiple(t *testing.T) {
	c := newFakeClientWithIPPools(
		makeIPPool("default-ipv4-ippool", "10.244.0.0/16"),
		makeIPPool("vlan100-pool", "10.100.0.0/24"),
		makeIPPool("vlan200-pool", "10.200.0.0/24"),
	).Build()

	pools, err := ListIPPools(context.Background(), c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pools) != 3 {
		t.Fatalf("got %d pools, want 3", len(pools))
	}
	want := map[string]string{
		"default-ipv4-ippool": "10.244.0.0/16",
		"vlan100-pool":        "10.100.0.0/24",
		"vlan200-pool":        "10.200.0.0/24",
	}
	for _, p := range pools {
		if want[p.Name] != p.CIDR {
			t.Errorf("pool %q CIDR = %q, want %q", p.Name, p.CIDR, want[p.Name])
		}
	}
}

func TestHasEligiblePool(t *testing.T) {
	tests := []struct {
		name        string
		pools       []IPPool
		vlanSubnets []string
		want        bool
	}{
		{
			name:        "PoolContainedInVLANSubnet",
			pools:       []IPPool{{CIDR: "10.100.0.0/24"}},
			vlanSubnets: []string{"10.100.0.0/24"},
			want:        true,
		},
		{
			name:        "PoolStrictlyContainedInVLANSubnet",
			pools:       []IPPool{{CIDR: "10.100.0.128/25"}},
			vlanSubnets: []string{"10.100.0.0/24"},
			want:        true,
		},
		{
			name:        "PoolLargerThanVLANSubnet",
			pools:       []IPPool{{CIDR: "10.0.0.0/8"}},
			vlanSubnets: []string{"10.100.0.0/24"},
			want:        false,
		},
		{
			name:        "PoolOnDifferentNetwork",
			pools:       []IPPool{{CIDR: "10.244.0.0/16"}},
			vlanSubnets: []string{"10.100.0.0/24"},
			want:        false,
		},
		{
			name:        "AtLeastOnePoolMatches",
			pools:       []IPPool{{CIDR: "10.244.0.0/16"}, {CIDR: "10.100.0.0/24"}},
			vlanSubnets: []string{"10.100.0.0/24"},
			want:        true,
		},
		{
			name:        "MultipleVLANSubnets",
			pools:       []IPPool{{CIDR: "10.200.0.0/24"}},
			vlanSubnets: []string{"10.100.0.0/24", "10.200.0.0/24"},
			want:        true,
		},
		{
			name:        "NoPools",
			pools:       nil,
			vlanSubnets: []string{"10.100.0.0/24"},
			want:        false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasEligiblePool(tt.pools, tt.vlanSubnets); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEligiblePoolForIP(t *testing.T) {
	pools := []IPPool{
		{Name: "default-ipv4-ippool", CIDR: "10.244.0.0/16"}, // cluster default, not in VLAN
		{Name: "vlan100-pool", CIDR: "10.100.0.0/24"},        // matches VLAN 100 subnet exactly
		{Name: "vlan100-subpool", CIDR: "10.100.0.128/25"},   // contained within VLAN 100 subnet
		{Name: "vlan200-pool", CIDR: "10.200.0.0/24"},        // matches VLAN 200 subnet
	}

	tests := []struct {
		name        string
		ip          string
		vlanSubnets []string
		wantPool    string // empty means expect nil
	}{
		{
			name:        "IPInExactlyMatchingPool",
			ip:          "10.100.0.5",
			vlanSubnets: []string{"10.100.0.0/24"},
			wantPool:    "vlan100-pool",
		},
		{
			name:        "IPInSubpoolWithinVLAN",
			ip:          "10.100.0.200",
			vlanSubnets: []string{"10.100.0.0/24"},
			// Either vlan100-pool or vlan100-subpool covers it; first match wins.
			wantPool: "vlan100-pool",
		},
		{
			name:        "IPNotInVLANSubnet",
			ip:          "10.244.5.1", // in cluster default pool but VLAN list excludes it
			vlanSubnets: []string{"10.100.0.0/24"},
			wantPool:    "",
		},
		{
			name:        "PoolNotContainedInVLAN",
			ip:          "10.100.0.5",
			vlanSubnets: []string{"10.100.0.0/26"}, // VLAN is smaller than vlan100-pool
			wantPool:    "",
		},
		{
			name:        "MultipleVLANSubnets",
			ip:          "10.200.0.5",
			vlanSubnets: []string{"10.100.0.0/24", "10.200.0.0/24"},
			wantPool:    "vlan200-pool",
		},
		{
			name:        "InvalidIP",
			ip:          "not-an-ip",
			vlanSubnets: []string{"10.100.0.0/24"},
			wantPool:    "",
		},
		{
			name:        "NoPools",
			ip:          "10.100.0.5",
			vlanSubnets: []string{"10.100.0.0/24"},
			wantPool:    "", // pools list is empty in this branch by override below
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			poolList := pools
			if tt.name == "NoPools" {
				poolList = nil
			}
			got := EligiblePoolForIP(poolList, tt.ip, tt.vlanSubnets)
			if tt.wantPool == "" {
				if got != nil {
					t.Errorf("got pool %+v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("got nil pool, want %q", tt.wantPool)
			}
			if got.Name != tt.wantPool {
				t.Errorf("got pool %q, want %q", got.Name, tt.wantPool)
			}
		})
	}
}
