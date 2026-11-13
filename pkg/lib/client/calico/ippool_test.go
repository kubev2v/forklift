package calico

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

func TestParseIPPool_Multiple(t *testing.T) {
	pools := []IPPool{}
	for _, u := range []*unstructured.Unstructured{
		makeIPPool("default-ipv4-ippool", "10.244.0.0/16"),
		makeIPPool("vlan100-pool", "10.100.0.0/24"),
		makeIPPool("vlan200-pool", "10.200.0.0/24"),
	} {
		pool, err := ParseIPPool(u)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		pools = append(pools, *pool)
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

func TestParseIPPool_DisabledAndAllowedUses(t *testing.T) {
	byName := map[string]IPPool{}
	for _, u := range []*unstructured.Unstructured{
		makeIPPool("plain", "10.100.0.0/24"),
		makeIPPoolWithFields("disabled-pool", "10.101.0.0/24", true, false, nil),
		makeIPPoolWithFields("explicit-empty-uses", "10.102.0.0/24", false, true, []string{}),
		makeIPPoolWithFields("workload-only", "10.103.0.0/24", false, true, []string{"Workload"}),
		makeIPPoolWithFields("l2-only", "10.104.0.0/24", false, true, []string{"L2Workload"}),
	} {
		pool, err := ParseIPPool(u)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		byName[pool.Name] = *pool
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
}
