package base

import (
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
)

func TestValidateNetworkDuplicates_NilNetworkMap(t *testing.T) {
	foundNadDup, foundPodDup := ValidateNetworkDuplicates(nil, nil)
	if foundNadDup || foundPodDup {
		t.Errorf("nil map should return (false, false), got (%v, %v)", foundNadDup, foundPodDup)
	}
}

func TestValidateNetworkDuplicates_NoNICs(t *testing.T) {
	nm := &api.NetworkMap{Spec: api.NetworkMapSpec{Map: []api.NetworkPair{}}}
	foundNadDup, foundPodDup := ValidateNetworkDuplicates(nil, nm)
	if foundNadDup || foundPodDup {
		t.Errorf("empty NIC list should find no duplicates, got (%v, %v)", foundNadDup, foundPodDup)
	}
}

func TestValidateNetworkDuplicates_SinglePod(t *testing.T) {
	nm := &api.NetworkMap{Spec: api.NetworkMapSpec{Map: []api.NetworkPair{
		{Source: api.NetworkSourceRef{Ref: ref.Ref{Type: "pod"}}, Destination: api.DestinationNetwork{Type: Pod}},
	}}}
	nicRefs := []ref.Ref{{Type: "pod"}}
	foundNadDup, foundPodDup := ValidateNetworkDuplicates(nicRefs, nm)
	if foundNadDup || foundPodDup {
		t.Errorf("single pod NIC should find no duplicates, got (%v, %v)", foundNadDup, foundPodDup)
	}
}

func TestValidateNetworkDuplicates_DuplicatePod(t *testing.T) {
	nm := &api.NetworkMap{Spec: api.NetworkMapSpec{Map: []api.NetworkPair{
		{Source: api.NetworkSourceRef{Ref: ref.Ref{Type: "pod"}}, Destination: api.DestinationNetwork{Type: Pod}},
		{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-1"}}, Destination: api.DestinationNetwork{Type: Pod}},
	}}}
	nicRefs := []ref.Ref{{Type: "pod"}, {ID: "net-1"}}
	foundNadDup, foundPodDup := ValidateNetworkDuplicates(nicRefs, nm)
	if foundNadDup {
		t.Errorf("no NAD duplicates expected, got foundNadDup=true")
	}
	if !foundPodDup {
		t.Errorf("two NICs mapped to pod should detect duplicate, got foundPodDup=false")
	}
}

func TestValidateNetworkDuplicates_DuplicateNAD_ByID(t *testing.T) {
	nm := &api.NetworkMap{Spec: api.NetworkMapSpec{Map: []api.NetworkPair{
		{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-1"}}, Destination: api.DestinationNetwork{Type: Multus, Namespace: "ns", Name: "nad-a"}},
	}}}
	nicRefs := []ref.Ref{{ID: "net-1"}, {ID: "net-1"}}
	foundNadDup, foundPodDup := ValidateNetworkDuplicates(nicRefs, nm)
	if !foundNadDup {
		t.Errorf("duplicate NAD (same source ID) should be detected, got foundNadDup=false")
	}
	if foundPodDup {
		t.Errorf("no pod mapping, foundPodDup should be false, got true")
	}
}

func TestValidateNetworkDuplicates_DuplicateNAD_ByNameNamespace(t *testing.T) {
	nm := &api.NetworkMap{Spec: api.NetworkMapSpec{Map: []api.NetworkPair{
		{Source: api.NetworkSourceRef{Ref: ref.Ref{Namespace: "ns", Name: "net-1"}}, Destination: api.DestinationNetwork{Type: Multus, Namespace: "ns", Name: "nad-a"}},
		{Source: api.NetworkSourceRef{Ref: ref.Ref{Namespace: "ns", Name: "net-2"}}, Destination: api.DestinationNetwork{Type: Multus, Namespace: "ns", Name: "nad-a"}},
	}}}
	nicRefs := []ref.Ref{{Namespace: "ns", Name: "net-1"}, {Namespace: "ns", Name: "net-2"}}
	foundNadDup, _ := ValidateNetworkDuplicates(nicRefs, nm)
	if !foundNadDup {
		t.Errorf("two NIC refs mapped to same NAD should be detected, got foundNadDup=false")
	}
}

func TestValidateNetworkDuplicates_DistinctNADs(t *testing.T) {
	nm := &api.NetworkMap{Spec: api.NetworkMapSpec{Map: []api.NetworkPair{
		{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-1"}}, Destination: api.DestinationNetwork{Type: Multus, Namespace: "ns", Name: "nad-a"}},
		{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-2"}}, Destination: api.DestinationNetwork{Type: Multus, Namespace: "ns", Name: "nad-b"}},
	}}}
	nicRefs := []ref.Ref{{ID: "net-1"}, {ID: "net-2"}}
	foundNadDup, foundPodDup := ValidateNetworkDuplicates(nicRefs, nm)
	if foundNadDup || foundPodDup {
		t.Errorf("distinct NADs should find no duplicates, got (%v, %v)", foundNadDup, foundPodDup)
	}
}

func TestValidateNetworkDuplicates_UnmappedNICIgnored(t *testing.T) {
	nm := &api.NetworkMap{Spec: api.NetworkMapSpec{Map: []api.NetworkPair{
		{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-1"}}, Destination: api.DestinationNetwork{Type: Multus, Namespace: "ns", Name: "nad-a"}},
	}}}
	nicRefs := []ref.Ref{{ID: "net-1"}, {ID: "net-999"}}
	foundNadDup, foundPodDup := ValidateNetworkDuplicates(nicRefs, nm)
	if foundNadDup || foundPodDup {
		t.Errorf("unmapped NIC should be ignored, got (%v, %v)", foundNadDup, foundPodDup)
	}
}

// --- FindAllMappingsForNICRef ---

func TestFindAllMappingsForNICRef_MultipleByID(t *testing.T) {
	nm := &api.NetworkMap{Spec: api.NetworkMapSpec{Map: []api.NetworkPair{
		{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-1"}}, Destination: api.DestinationNetwork{Type: Multus, Namespace: "ns", Name: "nad-a"}},
		{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-1"}}, Destination: api.DestinationNetwork{Type: Multus, Namespace: "ns", Name: "nad-b"}},
		{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-2"}}, Destination: api.DestinationNetwork{Type: Multus, Namespace: "ns", Name: "nad-c"}},
	}}}
	pairs := FindAllMappingsForNICRef(ref.Ref{ID: "net-1"}, nm)
	if len(pairs) != 2 {
		t.Fatalf("expected 2 pairs, got %d", len(pairs))
	}
	if pairs[0].Destination.Name != "nad-a" || pairs[1].Destination.Name != "nad-b" {
		t.Errorf("unexpected destinations: %v, %v", pairs[0].Destination.Name, pairs[1].Destination.Name)
	}
}

func TestFindAllMappingsForNICRef_SingleMatch(t *testing.T) {
	nm := &api.NetworkMap{Spec: api.NetworkMapSpec{Map: []api.NetworkPair{
		{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-1"}}, Destination: api.DestinationNetwork{Type: Multus, Namespace: "ns", Name: "nad-a"}},
	}}}
	pairs := FindAllMappingsForNICRef(ref.Ref{ID: "net-1"}, nm)
	if len(pairs) != 1 {
		t.Fatalf("expected 1 pair, got %d", len(pairs))
	}
}

func TestFindAllMappingsForNICRef_NilMap(t *testing.T) {
	pairs := FindAllMappingsForNICRef(ref.Ref{ID: "net-1"}, nil)
	if pairs != nil {
		t.Errorf("expected nil, got %v", pairs)
	}
}

func TestFindAllMappingsForNICRef_NoMatch(t *testing.T) {
	nm := &api.NetworkMap{Spec: api.NetworkMapSpec{Map: []api.NetworkPair{
		{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-1"}}, Destination: api.DestinationNetwork{Type: Multus, Namespace: "ns", Name: "nad-a"}},
	}}}
	pairs := FindAllMappingsForNICRef(ref.Ref{ID: "net-999"}, nm)
	if len(pairs) != 0 {
		t.Errorf("expected 0 pairs, got %d", len(pairs))
	}
}

// --- NADPool ---

func TestNADPool_Allocate_DistinctNADs(t *testing.T) {
	pairsForSource := []api.NetworkPair{
		{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-1"}}, Destination: api.DestinationNetwork{Type: Multus, Namespace: "ns", Name: "nad-a"}},
		{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-1"}}, Destination: api.DestinationNetwork{Type: Multus, Namespace: "ns", Name: "nad-b"}},
	}
	pool := NewNADPool()

	pair1, allocated1 := pool.Allocate(pairsForSource)
	pair2, allocated2 := pool.Allocate(pairsForSource)

	if !allocated1 || !allocated2 {
		t.Fatal("both allocations should succeed")
	}
	if pair1.Destination.Name == pair2.Destination.Name {
		t.Errorf("expected distinct NADs, both got %s", pair1.Destination.Name)
	}
}

func TestNADPool_Allocate_PoolExhausted(t *testing.T) {
	pairsForSource := []api.NetworkPair{
		{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-1"}}, Destination: api.DestinationNetwork{Type: Multus, Namespace: "ns", Name: "nad-a"}},
	}
	pool := NewNADPool()

	_, allocated1 := pool.Allocate(pairsForSource)
	_, allocated2 := pool.Allocate(pairsForSource)

	if !allocated1 {
		t.Error("first allocation should succeed")
	}
	if allocated2 {
		t.Error("second allocation should fail (pool exhausted)")
	}
}

func TestAllocateNetwork_PodPassthrough(t *testing.T) {
	pairsForSource := []api.NetworkPair{
		{Source: api.NetworkSourceRef{Ref: ref.Ref{Type: "pod"}}, Destination: api.DestinationNetwork{Type: Pod}},
	}
	pool := NewNADPool()

	pair, allocated := AllocateNetwork(pool, pairsForSource)
	if !allocated {
		t.Fatal("pod allocation should succeed")
	}
	if pair.Destination.Type != Pod {
		t.Errorf("expected pod type, got %s", pair.Destination.Type)
	}
}

func TestAllocateNetwork_MultusGoesToPool(t *testing.T) {
	pairsForSource := []api.NetworkPair{
		{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-1"}}, Destination: api.DestinationNetwork{Type: Multus, Namespace: "ns", Name: "nad-a"}},
		{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-1"}}, Destination: api.DestinationNetwork{Type: Multus, Namespace: "ns", Name: "nad-b"}},
	}
	pool := NewNADPool()

	pair1, allocated1 := AllocateNetwork(pool, pairsForSource)
	pair2, allocated2 := AllocateNetwork(pool, pairsForSource)
	if !allocated1 || !allocated2 {
		t.Fatal("both allocations should succeed")
	}
	if pair1.Destination.Name == pair2.Destination.Name {
		t.Errorf("expected distinct NADs, both got %s", pair1.Destination.Name)
	}
}

func TestNADPool_Allocate_Empty(t *testing.T) {
	pool := NewNADPool()
	_, allocated := pool.Allocate(nil)
	if allocated {
		t.Error("empty pairs should return false")
	}
}

func TestNADPool_Allocate_IndependentNetworks(t *testing.T) {
	pairsForSourceA := []api.NetworkPair{
		{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-1"}}, Destination: api.DestinationNetwork{Type: Multus, Namespace: "ns", Name: "nad-a"}},
	}
	pairsForSourceB := []api.NetworkPair{
		{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-2"}}, Destination: api.DestinationNetwork{Type: Multus, Namespace: "ns", Name: "nad-b"}},
	}
	pool := NewNADPool()

	pair1, allocated1 := pool.Allocate(pairsForSourceA)
	pair2, allocated2 := pool.Allocate(pairsForSourceB)

	if !allocated1 || !allocated2 {
		t.Fatal("both should succeed for independent networks")
	}
	if pair1.Destination.Name != "nad-a" || pair2.Destination.Name != "nad-b" {
		t.Errorf("unexpected assignments: %s, %s", pair1.Destination.Name, pair2.Destination.Name)
	}
}

// --- ValidateNetworkDuplicates with NAD pool ---

func TestValidateNetworkDuplicates_1toN_NoDuplicate(t *testing.T) {
	nm := &api.NetworkMap{Spec: api.NetworkMapSpec{Map: []api.NetworkPair{
		{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-1"}}, Destination: api.DestinationNetwork{Type: Multus, Namespace: "ns", Name: "nad-a"}},
		{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-1"}}, Destination: api.DestinationNetwork{Type: Multus, Namespace: "ns", Name: "nad-b"}},
	}}}
	nicRefs := []ref.Ref{{ID: "net-1"}, {ID: "net-1"}}
	foundNadDup, foundPodDup := ValidateNetworkDuplicates(nicRefs, nm)
	if foundNadDup {
		t.Error("with 2 NADs for 2 NICs, should not flag duplicate")
	}
	if foundPodDup {
		t.Error("no pod mapping, should not flag pod duplicate")
	}
}

func TestValidateNetworkDuplicates_1toN_PoolExhausted(t *testing.T) {
	nm := &api.NetworkMap{Spec: api.NetworkMapSpec{Map: []api.NetworkPair{
		{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-1"}}, Destination: api.DestinationNetwork{Type: Multus, Namespace: "ns", Name: "nad-a"}},
		{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-1"}}, Destination: api.DestinationNetwork{Type: Multus, Namespace: "ns", Name: "nad-b"}},
	}}}
	nicRefs := []ref.Ref{{ID: "net-1"}, {ID: "net-1"}, {ID: "net-1"}}
	foundNadDup, _ := ValidateNetworkDuplicates(nicRefs, nm)
	if !foundNadDup {
		t.Error("3 NICs with only 2 NADs should flag duplicate")
	}
}

func TestValidateNetworkDuplicates_1toN_MixedNetworks(t *testing.T) {
	nm := &api.NetworkMap{Spec: api.NetworkMapSpec{Map: []api.NetworkPair{
		{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-1"}}, Destination: api.DestinationNetwork{Type: Multus, Namespace: "ns", Name: "nad-a"}},
		{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-1"}}, Destination: api.DestinationNetwork{Type: Multus, Namespace: "ns", Name: "nad-b"}},
		{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-2"}}, Destination: api.DestinationNetwork{Type: Multus, Namespace: "ns", Name: "nad-c"}},
	}}}
	nicRefs := []ref.Ref{{ID: "net-1"}, {ID: "net-1"}, {ID: "net-2"}}
	foundNadDup, foundPodDup := ValidateNetworkDuplicates(nicRefs, nm)
	if foundNadDup || foundPodDup {
		t.Errorf("sufficient NADs for all NICs, should find no duplicates, got (%v, %v)", foundNadDup, foundPodDup)
	}
}

// --- ResolveNICModes ---

func TestResolveNICModes_NilNetworkMap_PreserveTrue(t *testing.T) {
	nics := []NICRef{{MAC: "aa:bb:cc:dd:ee:01", NetworkID: "net-1"}}
	modes := ResolveNICModes(nics, nil, true)
	if modes["aa:bb:cc:dd:ee:01"] != "preserve" {
		t.Errorf("nil network map with preserveStaticIPs=true should fallback to preserve, got %q", modes["aa:bb:cc:dd:ee:01"])
	}
}

func TestResolveNICModes_NilNetworkMap_PreserveFalse(t *testing.T) {
	nics := []NICRef{{MAC: "aa:bb:cc:dd:ee:01", NetworkID: "net-1"}}
	modes := ResolveNICModes(nics, nil, false)
	if len(modes) != 0 {
		t.Errorf("nil network map with preserveStaticIPs=false should return empty, got %v", modes)
	}
}

func TestResolveNICModes_EmptyNetworkIPMode_PreserveTrue(t *testing.T) {
	nm := &api.NetworkMap{Spec: api.NetworkMapSpec{Map: []api.NetworkPair{
		{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-1"}}, Destination: api.DestinationNetwork{Type: Pod}},
	}}}
	nics := []NICRef{{MAC: "aa:bb:cc:dd:ee:01", NetworkID: "net-1"}}
	modes := ResolveNICModes(nics, nm, true)
	if modes["aa:bb:cc:dd:ee:01"] != "preserve" {
		t.Errorf("expected 'preserve', got %q", modes["aa:bb:cc:dd:ee:01"])
	}
}

func TestResolveNICModes_EmptyNetworkIPMode_PreserveFalse(t *testing.T) {
	nm := &api.NetworkMap{Spec: api.NetworkMapSpec{Map: []api.NetworkPair{
		{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-1"}}, Destination: api.DestinationNetwork{Type: Pod}},
	}}}
	nics := []NICRef{{MAC: "aa:bb:cc:dd:ee:01", NetworkID: "net-1"}}
	modes := ResolveNICModes(nics, nm, false)
	if modes["aa:bb:cc:dd:ee:01"] != "none" {
		t.Errorf("expected 'none', got %q", modes["aa:bb:cc:dd:ee:01"])
	}
}

func TestResolveNICModes_NetworkIPModeOverridesPlanLevel(t *testing.T) {
	nm := &api.NetworkMap{Spec: api.NetworkMapSpec{Map: []api.NetworkPair{
		{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-1"}}, Destination: api.DestinationNetwork{Type: Pod}, NetworkIPMode: api.NetworkIPModeDHCP},
	}}}
	nics := []NICRef{{MAC: "aa:bb:cc:dd:ee:01", NetworkID: "net-1"}}
	modes := ResolveNICModes(nics, nm, true)
	if modes["aa:bb:cc:dd:ee:01"] != "dhcp" {
		t.Errorf("networkIPMode should override plan-level, expected 'dhcp', got %q", modes["aa:bb:cc:dd:ee:01"])
	}
}

func TestResolveNICModes_PreserveOverridesPreserveFalse(t *testing.T) {
	nm := &api.NetworkMap{Spec: api.NetworkMapSpec{Map: []api.NetworkPair{
		{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-1"}}, Destination: api.DestinationNetwork{Type: Pod}, NetworkIPMode: api.NetworkIPModePreserve},
	}}}
	nics := []NICRef{{MAC: "aa:bb:cc:dd:ee:01", NetworkID: "net-1"}}
	modes := ResolveNICModes(nics, nm, false)
	if modes["aa:bb:cc:dd:ee:01"] != "preserve" {
		t.Errorf("networkIPMode=preserve should override preserveStaticIPs=false, got %q", modes["aa:bb:cc:dd:ee:01"])
	}
}

func TestResolveNICModes_UnmappedNICSkipped(t *testing.T) {
	nm := &api.NetworkMap{Spec: api.NetworkMapSpec{Map: []api.NetworkPair{
		{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-1"}}, Destination: api.DestinationNetwork{Type: Pod}},
	}}}
	nics := []NICRef{
		{MAC: "aa:bb:cc:dd:ee:01", NetworkID: "net-1"},
		{MAC: "aa:bb:cc:dd:ee:02", NetworkID: "net-unmapped"},
	}
	modes := ResolveNICModes(nics, nm, true)
	if len(modes) != 1 {
		t.Errorf("expected 1 entry, got %d", len(modes))
	}
	if _, ok := modes["aa:bb:cc:dd:ee:02"]; ok {
		t.Error("unmapped NIC should not appear in modes map")
	}
}

func TestResolveNICModes_MixedModes(t *testing.T) {
	nm := &api.NetworkMap{Spec: api.NetworkMapSpec{Map: []api.NetworkPair{
		{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-1"}}, Destination: api.DestinationNetwork{Type: Pod}, NetworkIPMode: api.NetworkIPModePreserve},
		{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-2"}}, Destination: api.DestinationNetwork{Type: Multus, Name: "br0", Namespace: "ns"}, NetworkIPMode: api.NetworkIPModeDHCP},
		{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-3"}}, Destination: api.DestinationNetwork{Type: Multus, Name: "br1", Namespace: "ns"}, NetworkIPMode: api.NetworkIPModeNone},
		{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-4"}}, Destination: api.DestinationNetwork{Type: Pod}},
	}}}
	nics := []NICRef{
		{MAC: "mac-1", NetworkID: "net-1"},
		{MAC: "mac-2", NetworkID: "net-2"},
		{MAC: "mac-3", NetworkID: "net-3"},
		{MAC: "mac-4", NetworkID: "net-4"},
	}
	modes := ResolveNICModes(nics, nm, true)
	if modes["mac-1"] != "preserve" {
		t.Errorf("mac-1: expected 'preserve', got %q", modes["mac-1"])
	}
	if modes["mac-2"] != "dhcp" {
		t.Errorf("mac-2: expected 'dhcp', got %q", modes["mac-2"])
	}
	if modes["mac-3"] != "none" {
		t.Errorf("mac-3: expected 'none', got %q", modes["mac-3"])
	}
	if modes["mac-4"] != "preserve" {
		t.Errorf("mac-4: expected 'preserve' (plan-level fallback), got %q", modes["mac-4"])
	}
}

// --- ResolveNICModes 1:N allocation ---

func TestResolveNICModes_1toN_DifferentNetworkIPMode(t *testing.T) {
	// Two NICs on the same source network, mapped to two different NADs
	// with different networkIPMode values. Each NIC should get its own pair's mode.
	nm := &api.NetworkMap{Spec: api.NetworkMapSpec{Map: []api.NetworkPair{
		{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-1"}}, Destination: api.DestinationNetwork{Type: Multus, Namespace: "ns", Name: "nad-a"}, NetworkIPMode: api.NetworkIPModePreserve},
		{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-1"}}, Destination: api.DestinationNetwork{Type: Multus, Namespace: "ns", Name: "nad-b"}, NetworkIPMode: api.NetworkIPModeNone},
	}}}
	nics := []NICRef{
		{MAC: "mac-1", NetworkID: "net-1"},
		{MAC: "mac-2", NetworkID: "net-1"},
	}
	modes := ResolveNICModes(nics, nm, true)
	if modes["mac-1"] != "preserve" {
		t.Errorf("mac-1: expected 'preserve' (from row 0), got %q", modes["mac-1"])
	}
	if modes["mac-2"] != "none" {
		t.Errorf("mac-2: expected 'none' (from row 1), got %q", modes["mac-2"])
	}
}

func TestResolveNICModes_1toN_PoolExhausted(t *testing.T) {
	// 3 NICs on the same source network but only 2 NAD rows — third NIC should be skipped.
	nm := &api.NetworkMap{Spec: api.NetworkMapSpec{Map: []api.NetworkPair{
		{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-1"}}, Destination: api.DestinationNetwork{Type: Multus, Namespace: "ns", Name: "nad-a"}, NetworkIPMode: api.NetworkIPModePreserve},
		{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-1"}}, Destination: api.DestinationNetwork{Type: Multus, Namespace: "ns", Name: "nad-b"}, NetworkIPMode: api.NetworkIPModeDHCP},
	}}}
	nics := []NICRef{
		{MAC: "mac-1", NetworkID: "net-1"},
		{MAC: "mac-2", NetworkID: "net-1"},
		{MAC: "mac-3", NetworkID: "net-1"},
	}
	modes := ResolveNICModes(nics, nm, true)
	if modes["mac-1"] != "preserve" {
		t.Errorf("mac-1: expected 'preserve', got %q", modes["mac-1"])
	}
	if modes["mac-2"] != "dhcp" {
		t.Errorf("mac-2: expected 'dhcp', got %q", modes["mac-2"])
	}
	if _, exists := modes["mac-3"]; exists {
		t.Errorf("mac-3: expected not allocated (pool exhausted), got %q", modes["mac-3"])
	}
}

func TestResolveNICModes_1toN_MixedNetworks(t *testing.T) {
	// Two NICs on net-1 (1:N) and one NIC on net-2 (1:1).
	nm := &api.NetworkMap{Spec: api.NetworkMapSpec{Map: []api.NetworkPair{
		{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-1"}}, Destination: api.DestinationNetwork{Type: Multus, Namespace: "ns", Name: "nad-a"}, NetworkIPMode: api.NetworkIPModePreserve},
		{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-1"}}, Destination: api.DestinationNetwork{Type: Multus, Namespace: "ns", Name: "nad-b"}, NetworkIPMode: api.NetworkIPModeNone},
		{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-2"}}, Destination: api.DestinationNetwork{Type: Multus, Namespace: "ns", Name: "nad-c"}, NetworkIPMode: api.NetworkIPModeDHCP},
	}}}
	nics := []NICRef{
		{MAC: "mac-1", NetworkID: "net-1"},
		{MAC: "mac-2", NetworkID: "net-1"},
		{MAC: "mac-3", NetworkID: "net-2"},
	}
	modes := ResolveNICModes(nics, nm, false)
	if modes["mac-1"] != "preserve" {
		t.Errorf("mac-1: expected 'preserve', got %q", modes["mac-1"])
	}
	if modes["mac-2"] != "none" {
		t.Errorf("mac-2: expected 'none', got %q", modes["mac-2"])
	}
	if modes["mac-3"] != "dhcp" {
		t.Errorf("mac-3: expected 'dhcp', got %q", modes["mac-3"])
	}
}

// --- HasPreserveMode ---

func TestHasPreserveMode_True(t *testing.T) {
	modes := map[string]string{"mac-1": "none", "mac-2": "preserve"}
	if !HasPreserveMode(modes) {
		t.Error("expected true when at least one NIC is preserve")
	}
}

func TestHasPreserveMode_False(t *testing.T) {
	modes := map[string]string{"mac-1": "none", "mac-2": "dhcp"}
	if HasPreserveMode(modes) {
		t.Error("expected false when no NIC is preserve")
	}
}

func TestHasPreserveMode_Empty(t *testing.T) {
	if HasPreserveMode(map[string]string{}) {
		t.Error("expected false for empty map")
	}
}

func TestHasPreserveMode_Nil(t *testing.T) {
	if HasPreserveMode(nil) {
		t.Error("expected false for nil map")
	}
}

func TestValidateNetworkDuplicates_BackwardCompat_SingleRow(t *testing.T) {
	nm := &api.NetworkMap{Spec: api.NetworkMapSpec{Map: []api.NetworkPair{
		{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-1"}}, Destination: api.DestinationNetwork{Type: Multus, Namespace: "ns", Name: "nad-a"}},
	}}}
	nicRefs := []ref.Ref{{ID: "net-1"}, {ID: "net-1"}}
	foundNadDup, _ := ValidateNetworkDuplicates(nicRefs, nm)
	if !foundNadDup {
		t.Error("single-row map with 2 NICs should still flag duplicate (backward compatible)")
	}
}
