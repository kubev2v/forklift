package base

import (
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
)

func TestValidatePodNetworkDuplicates_NilNetworkMap(t *testing.T) {
	if ValidatePodNetworkDuplicates(nil, nil) {
		t.Error("nil map should return false")
	}
}

func TestValidatePodNetworkDuplicates_NoNICs(t *testing.T) {
	nm := &api.NetworkMap{Spec: api.NetworkMapSpec{Map: []api.NetworkPair{}}}
	if ValidatePodNetworkDuplicates(nil, nm) {
		t.Error("empty NIC list should find no duplicates")
	}
}

func TestValidatePodNetworkDuplicates_SinglePod(t *testing.T) {
	nm := &api.NetworkMap{Spec: api.NetworkMapSpec{Map: []api.NetworkPair{
		{Source: api.NetworkSourceRef{Ref: ref.Ref{Type: "pod"}}, Destination: api.DestinationNetwork{Type: Pod}},
	}}}
	nicRefs := []ref.Ref{{Type: "pod"}}
	if ValidatePodNetworkDuplicates(nicRefs, nm) {
		t.Error("single pod NIC should find no duplicates")
	}
}

func TestValidatePodNetworkDuplicates_DuplicatePod(t *testing.T) {
	nm := &api.NetworkMap{Spec: api.NetworkMapSpec{Map: []api.NetworkPair{
		{Source: api.NetworkSourceRef{Ref: ref.Ref{Type: "pod"}}, Destination: api.DestinationNetwork{Type: Pod}},
		{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-1"}}, Destination: api.DestinationNetwork{Type: Pod}},
	}}}
	nicRefs := []ref.Ref{{Type: "pod"}, {ID: "net-1"}}
	if !ValidatePodNetworkDuplicates(nicRefs, nm) {
		t.Error("two NICs mapped to pod should detect duplicate")
	}
}

func TestValidatePodNetworkDuplicates_SameNADAllowed(t *testing.T) {
	nm := &api.NetworkMap{Spec: api.NetworkMapSpec{Map: []api.NetworkPair{
		{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-1"}}, Destination: api.DestinationNetwork{Type: Multus, Namespace: "ns", Name: "nad-a"}},
	}}}
	nicRefs := []ref.Ref{{ID: "net-1"}, {ID: "net-1"}}
	if ValidatePodNetworkDuplicates(nicRefs, nm) {
		t.Error("duplicate NAD on same source should not be flagged")
	}
}

func TestValidatePodNetworkDuplicates_DistinctNADs(t *testing.T) {
	nm := &api.NetworkMap{Spec: api.NetworkMapSpec{Map: []api.NetworkPair{
		{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-1"}}, Destination: api.DestinationNetwork{Type: Multus, Namespace: "ns", Name: "nad-a"}},
		{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-2"}}, Destination: api.DestinationNetwork{Type: Multus, Namespace: "ns", Name: "nad-b"}},
	}}}
	nicRefs := []ref.Ref{{ID: "net-1"}, {ID: "net-2"}}
	if ValidatePodNetworkDuplicates(nicRefs, nm) {
		t.Error("distinct NADs should find no pod duplicates")
	}
}

func TestValidatePodNetworkDuplicates_UnmappedNICIgnored(t *testing.T) {
	nm := &api.NetworkMap{Spec: api.NetworkMapSpec{Map: []api.NetworkPair{
		{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-1"}}, Destination: api.DestinationNetwork{Type: Multus, Namespace: "ns", Name: "nad-a"}},
	}}}
	nicRefs := []ref.Ref{{ID: "net-1"}, {ID: "net-999"}}
	if ValidatePodNetworkDuplicates(nicRefs, nm) {
		t.Error("unmapped NIC should be ignored")
	}
}

func TestValidatePodNetworkDuplicates_MultusBeforePod(t *testing.T) {
	t.Run("single NIC with Multus then Pod resolves to pod without duplicate", func(t *testing.T) {
		nm := &api.NetworkMap{Spec: api.NetworkMapSpec{Map: []api.NetworkPair{
			{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-1"}}, Destination: api.DestinationNetwork{Type: Multus, Namespace: "ns", Name: "nad-a"}},
			{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-1"}}, Destination: api.DestinationNetwork{Type: Pod}},
		}}}
		nicRefs := []ref.Ref{{ID: "net-1"}}
		if ValidatePodNetworkDuplicates(nicRefs, nm) {
			t.Error("single NIC resolving to pod should not flag duplicate")
		}
	})

	t.Run("two NICs resolving to pod triggers VMMultiplePodNetworkMappings", func(t *testing.T) {
		nm := &api.NetworkMap{Spec: api.NetworkMapSpec{Map: []api.NetworkPair{
			{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-1"}}, Destination: api.DestinationNetwork{Type: Multus, Namespace: "ns", Name: "nad-a"}},
			{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-1"}}, Destination: api.DestinationNetwork{Type: Pod}},
			{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-2"}}, Destination: api.DestinationNetwork{Type: Pod}},
		}}}
		nicRefs := []ref.Ref{{ID: "net-1"}, {ID: "net-2"}}
		if !ValidatePodNetworkDuplicates(nicRefs, nm) {
			t.Error("two NICs mapped to pod should detect duplicate (VMMultiplePodNetworkMappings)")
		}
	})
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

func TestNADPool_Allocate_SameNADReused(t *testing.T) {
	pairsForSource := []api.NetworkPair{
		{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-1"}}, Destination: api.DestinationNetwork{Type: Multus, Namespace: "ns", Name: "nad-a"}},
	}
	pool := NewNADPool()

	pair1, allocated1 := pool.Allocate(pairsForSource)
	pair2, allocated2 := pool.Allocate(pairsForSource)

	if !allocated1 || !allocated2 {
		t.Fatal("both allocations should succeed when reusing the same NAD")
	}
	if pair1.Destination.Name != "nad-a" || pair2.Destination.Name != "nad-a" {
		t.Errorf("expected same NAD nad-a, got %s and %s", pair1.Destination.Name, pair2.Destination.Name)
	}
}

func TestNADPool_Allocate_PoolDistinctThenExhausted(t *testing.T) {
	pairsForSource := []api.NetworkPair{
		{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-1"}}, Destination: api.DestinationNetwork{Type: Multus, Namespace: "ns", Name: "nad-a"}},
		{Source: api.NetworkSourceRef{Ref: ref.Ref{ID: "net-1"}}, Destination: api.DestinationNetwork{Type: Multus, Namespace: "ns", Name: "nad-b"}},
	}
	pool := NewNADPool()

	_, allocated1 := pool.Allocate(pairsForSource)
	_, allocated2 := pool.Allocate(pairsForSource)
	_, allocated3 := pool.Allocate(pairsForSource)

	if !allocated1 || !allocated2 {
		t.Fatal("first two allocations should succeed")
	}
	if allocated3 {
		t.Error("third allocation should fail when pool is exhausted")
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
