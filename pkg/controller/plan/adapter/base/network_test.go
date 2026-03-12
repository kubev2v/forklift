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
		{Source: ref.Ref{Type: "pod"}, Destination: api.DestinationNetwork{Type: Pod}},
	}}}
	nicRefs := []ref.Ref{{Type: "pod"}}
	foundNadDup, foundPodDup := ValidateNetworkDuplicates(nicRefs, nm)
	if foundNadDup || foundPodDup {
		t.Errorf("single pod NIC should find no duplicates, got (%v, %v)", foundNadDup, foundPodDup)
	}
}

func TestValidateNetworkDuplicates_DuplicatePod(t *testing.T) {
	nm := &api.NetworkMap{Spec: api.NetworkMapSpec{Map: []api.NetworkPair{
		{Source: ref.Ref{Type: "pod"}, Destination: api.DestinationNetwork{Type: Pod}},
		{Source: ref.Ref{ID: "net-1"}, Destination: api.DestinationNetwork{Type: Pod}},
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
		{Source: ref.Ref{ID: "net-1"}, Destination: api.DestinationNetwork{Type: Multus, Namespace: "ns", Name: "nad-a"}},
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
		{Source: ref.Ref{Namespace: "ns", Name: "net-1"}, Destination: api.DestinationNetwork{Type: Multus, Namespace: "ns", Name: "nad-a"}},
		{Source: ref.Ref{Namespace: "ns", Name: "net-2"}, Destination: api.DestinationNetwork{Type: Multus, Namespace: "ns", Name: "nad-a"}},
	}}}
	nicRefs := []ref.Ref{{Namespace: "ns", Name: "net-1"}, {Namespace: "ns", Name: "net-2"}}
	foundNadDup, _ := ValidateNetworkDuplicates(nicRefs, nm)
	if !foundNadDup {
		t.Errorf("two NIC refs mapped to same NAD should be detected, got foundNadDup=false")
	}
}

func TestValidateNetworkDuplicates_DistinctNADs(t *testing.T) {
	nm := &api.NetworkMap{Spec: api.NetworkMapSpec{Map: []api.NetworkPair{
		{Source: ref.Ref{ID: "net-1"}, Destination: api.DestinationNetwork{Type: Multus, Namespace: "ns", Name: "nad-a"}},
		{Source: ref.Ref{ID: "net-2"}, Destination: api.DestinationNetwork{Type: Multus, Namespace: "ns", Name: "nad-b"}},
	}}}
	nicRefs := []ref.Ref{{ID: "net-1"}, {ID: "net-2"}}
	foundNadDup, foundPodDup := ValidateNetworkDuplicates(nicRefs, nm)
	if foundNadDup || foundPodDup {
		t.Errorf("distinct NADs should find no duplicates, got (%v, %v)", foundNadDup, foundPodDup)
	}
}

func TestValidateNetworkDuplicates_UnmappedNICIgnored(t *testing.T) {
	nm := &api.NetworkMap{Spec: api.NetworkMapSpec{Map: []api.NetworkPair{
		{Source: ref.Ref{ID: "net-1"}, Destination: api.DestinationNetwork{Type: Multus, Namespace: "ns", Name: "nad-a"}},
	}}}
	nicRefs := []ref.Ref{{ID: "net-1"}, {ID: "net-999"}}
	foundNadDup, foundPodDup := ValidateNetworkDuplicates(nicRefs, nm)
	if foundNadDup || foundPodDup {
		t.Errorf("unmapped NIC should be ignored, got (%v, %v)", foundNadDup, foundPodDup)
	}
}

func TestFindMappingForNICRef_ByID(t *testing.T) {
	nm := &api.NetworkMap{Spec: api.NetworkMapSpec{Map: []api.NetworkPair{
		{Source: ref.Ref{ID: "net-1"}, Destination: api.DestinationNetwork{Type: Multus, Namespace: "ns", Name: "nad-a"}},
	}}}
	pair, found := FindMappingForNICRef(ref.Ref{ID: "net-1"}, nm)
	if !found {
		t.Fatal("expected to find mapping by ID")
	}
	if pair.Destination.Name != "nad-a" {
		t.Errorf("unexpected destination name: %s", pair.Destination.Name)
	}
}

func TestFindMappingForNICRef_ByType(t *testing.T) {
	nm := &api.NetworkMap{Spec: api.NetworkMapSpec{Map: []api.NetworkPair{
		{Source: ref.Ref{Type: "pod"}, Destination: api.DestinationNetwork{Type: Pod}},
	}}}
	_, found := FindMappingForNICRef(ref.Ref{Type: "pod"}, nm)
	if !found {
		t.Error("expected to find mapping by Type")
	}
}

func TestFindMappingForNICRef_ByNameNamespace(t *testing.T) {
	nm := &api.NetworkMap{Spec: api.NetworkMapSpec{Map: []api.NetworkPair{
		{Source: ref.Ref{Namespace: "ns", Name: "net-1"}, Destination: api.DestinationNetwork{Type: Multus, Namespace: "ns", Name: "nad-a"}},
	}}}
	_, found := FindMappingForNICRef(ref.Ref{Namespace: "ns", Name: "net-1"}, nm)
	if !found {
		t.Error("expected to find mapping by Name/Namespace")
	}
}

func TestFindMappingForNICRef_NotFound(t *testing.T) {
	nm := &api.NetworkMap{Spec: api.NetworkMapSpec{Map: []api.NetworkPair{
		{Source: ref.Ref{ID: "net-1"}, Destination: api.DestinationNetwork{Type: Multus, Namespace: "ns", Name: "nad-a"}},
	}}}
	_, found := FindMappingForNICRef(ref.Ref{ID: "net-999"}, nm)
	if found {
		t.Error("expected no match for unknown ID")
	}
}
