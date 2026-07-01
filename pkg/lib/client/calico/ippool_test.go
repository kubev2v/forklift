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

// makeIPPoolWithFields builds an IPPool with optional disabled/allowedUses.
// allowedUsesPresent distinguishes "field absent" (nil) from "field present
// but empty" ([]) — the parser handles these differently.
func makeIPPoolWithFields(name, cidr string, disabled bool, allowedUsesPresent bool, allowedUses []string) *unstructured.Unstructured {
	u := makeIPPool(name, cidr)
	if disabled {
		_ = unstructured.SetNestedField(u.Object, true, "spec", "disabled")
	}
	if allowedUsesPresent {
		ifaces := make([]interface{}, len(allowedUses))
		for i, v := range allowedUses {
			ifaces[i] = v
		}
		_ = unstructured.SetNestedSlice(u.Object, ifaces, "spec", "allowedUses")
	}
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

func TestListIPPools_ParsesDisabledAndAllowedUses(t *testing.T) {
	c := newFakeClientWithIPPools(
		makeIPPool("plain", "10.100.0.0/24"),
		makeIPPoolWithFields("disabled-pool", "10.101.0.0/24", true, false, nil),
		makeIPPoolWithFields("explicit-empty-uses", "10.102.0.0/24", false, true, []string{}),
		makeIPPoolWithFields("workload-only", "10.103.0.0/24", false, true, []string{"Workload"}),
		makeIPPoolWithFields("l2-only", "10.104.0.0/24", false, true, []string{"L2Workload"}),
		makeIPPoolWithFields("mixed-uses", "10.105.0.0/24", false, true, []string{"Workload", "L2Workload"}),
	).Build()

	pools, err := ListIPPools(context.Background(), c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	byName := map[string]IPPool{}
	for _, p := range pools {
		byName[p.Name] = p
	}

	// Absent field → nil; present-but-empty → []string{}; populated → slice.
	if got := byName["plain"]; got.Disabled || got.AllowedUses != nil {
		t.Errorf("plain pool: Disabled=%v, AllowedUses=%v (want false, nil)", got.Disabled, got.AllowedUses)
	}
	if got := byName["disabled-pool"]; !got.Disabled {
		t.Errorf("disabled-pool: Disabled=%v (want true)", got.Disabled)
	}
	if got := byName["explicit-empty-uses"]; got.AllowedUses == nil || len(got.AllowedUses) != 0 {
		t.Errorf("explicit-empty-uses: AllowedUses=%v (want non-nil empty slice)", got.AllowedUses)
	}
	if got := byName["workload-only"]; len(got.AllowedUses) != 1 || got.AllowedUses[0] != "Workload" {
		t.Errorf("workload-only: AllowedUses=%v (want [Workload])", got.AllowedUses)
	}
	if got := byName["l2-only"]; len(got.AllowedUses) != 1 || got.AllowedUses[0] != AllowedUseL2Workload {
		t.Errorf("l2-only: AllowedUses=%v (want [L2Workload])", got.AllowedUses)
	}
	if got := byName["mixed-uses"]; len(got.AllowedUses) != 2 {
		t.Errorf("mixed-uses: AllowedUses=%v (want 2 entries)", got.AllowedUses)
	}
}

func TestL3EligiblePools(t *testing.T) {
	tests := []struct {
		name string
		pool IPPool
		want bool // whether the pool should appear in L3EligiblePools output
	}{
		{"AbsentAllowedUses_NotDisabled", IPPool{Name: "p", CIDR: "10.0.0.0/8", AllowedUses: nil}, true},
		{"ExplicitEmptyAllowedUses", IPPool{Name: "p", CIDR: "10.0.0.0/8", AllowedUses: []string{}}, false},
		{"WorkloadOnly", IPPool{Name: "p", CIDR: "10.0.0.0/8", AllowedUses: []string{"Workload"}}, true},
		{"TunnelOnly", IPPool{Name: "p", CIDR: "10.0.0.0/8", AllowedUses: []string{"Tunnel"}}, true},
		{"L2WorkloadOnly", IPPool{Name: "p", CIDR: "10.0.0.0/8", AllowedUses: []string{AllowedUseL2Workload}}, false},
		{"WorkloadPlusL2Workload", IPPool{Name: "p", CIDR: "10.0.0.0/8", AllowedUses: []string{"Workload", AllowedUseL2Workload}}, true},
		{"Disabled_AbsentUses", IPPool{Name: "p", CIDR: "10.0.0.0/8", Disabled: true, AllowedUses: nil}, false},
		{"Disabled_WorkloadOnly", IPPool{Name: "p", CIDR: "10.0.0.0/8", Disabled: true, AllowedUses: []string{"Workload"}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := L3EligiblePools([]IPPool{tt.pool})
			got := len(out) == 1
			if got != tt.want {
				t.Errorf("got eligible=%v, want %v (pool=%+v)", got, tt.want, tt.pool)
			}
		})
	}
}

func TestL3EligiblePoolForIP(t *testing.T) {
	pools := []IPPool{
		{Name: "default-pool", CIDR: "10.244.0.0/16"},                                 // L3-eligible (nil allowedUses)
		{Name: "disabled-pool", CIDR: "10.100.0.0/24", Disabled: true},                // disabled
		{Name: "l2-only-pool", CIDR: "10.200.0.0/24", AllowedUses: []string{"L2Workload"}}, // not L3-eligible
	}
	tests := []struct {
		name     string
		ip       string
		wantPool string
	}{
		{"IP in L3-eligible pool", "10.244.5.1", "default-pool"},
		{"IP in disabled pool only", "10.100.0.5", ""},
		{"IP in L2-only pool", "10.200.0.5", ""},
		{"IP outside all pools", "192.168.1.1", ""},
		{"Invalid IP", "not-an-ip", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := L3EligiblePoolForIP(pools, tt.ip)
			if tt.wantPool == "" {
				if got != nil {
					t.Errorf("got %+v, want nil", got)
				}
				return
			}
			if got == nil || got.Name != tt.wantPool {
				t.Errorf("got %v, want pool %q", got, tt.wantPool)
			}
		})
	}
}

func TestL2WorkloadEligiblePools(t *testing.T) {
	vlanSubnets := []string{"10.100.0.0/24"}
	tests := []struct {
		name string
		pool IPPool
		want bool
	}{
		{"L2Workload + contained CIDR", IPPool{Name: "p", CIDR: "10.100.0.0/24", AllowedUses: []string{"L2Workload"}}, true},
		{"L2Workload mixed + contained", IPPool{Name: "p", CIDR: "10.100.0.0/25", AllowedUses: []string{"Workload", "L2Workload"}}, true},
		{"L2Workload but CIDR outside subnet", IPPool{Name: "p", CIDR: "10.200.0.0/24", AllowedUses: []string{"L2Workload"}}, false},
		{"L2Workload but pool wider than subnet", IPPool{Name: "p", CIDR: "10.0.0.0/8", AllowedUses: []string{"L2Workload"}}, false},
		{"Workload only", IPPool{Name: "p", CIDR: "10.100.0.0/24", AllowedUses: []string{"Workload"}}, false},
		{"Absent AllowedUses", IPPool{Name: "p", CIDR: "10.100.0.0/24"}, false},
		{"Disabled L2Workload", IPPool{Name: "p", CIDR: "10.100.0.0/24", Disabled: true, AllowedUses: []string{"L2Workload"}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := L2WorkloadEligiblePools([]IPPool{tt.pool}, vlanSubnets)
			got := len(out) == 1
			if got != tt.want {
				t.Errorf("got eligible=%v, want %v (pool=%+v)", got, tt.want, tt.pool)
			}
		})
	}
}

func TestL2WorkloadEligiblePoolForIP(t *testing.T) {
	pools := []IPPool{
		{Name: "l2-vlan100", CIDR: "10.100.0.0/24", AllowedUses: []string{"L2Workload"}},
		{Name: "workload-vlan100", CIDR: "10.100.0.0/24", AllowedUses: []string{"Workload"}},
		{Name: "l2-disabled", CIDR: "10.101.0.0/24", Disabled: true, AllowedUses: []string{"L2Workload"}},
	}
	vlanSubnets := []string{"10.100.0.0/24"}
	tests := []struct {
		name     string
		ip       string
		wantPool string
	}{
		{"L2 pool covers IP", "10.100.0.5", "l2-vlan100"},
		{"Only Workload covers (not L2)", "10.100.0.5", "l2-vlan100"}, // l2-vlan100 wins by order
		{"IP outside all L2-eligible", "10.101.0.5", ""},               // l2-disabled is disabled
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := L2WorkloadEligiblePoolForIP(pools, tt.ip, vlanSubnets)
			if tt.wantPool == "" {
				if got != nil {
					t.Errorf("got %+v, want nil", got)
				}
				return
			}
			if got == nil || got.Name != tt.wantPool {
				t.Errorf("got %v, want pool %q", got, tt.wantPool)
			}
		})
	}
}
