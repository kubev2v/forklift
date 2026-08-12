package base

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
)

// Network destination types.
const (
	Pod     = "pod"
	Multus  = "multus"
	Ignored = "ignored"
)

// FindAllMappingsForNICRef returns all NetworkPairs whose Source matches the given NIC ref.
func FindAllMappingsForNICRef(nicRef ref.Ref, networkMap *api.NetworkMap) []api.NetworkPair {
	if networkMap == nil {
		return nil
	}
	if nicRef.ID != "" {
		return networkMap.FindAllNetworks(nicRef.ID)
	}
	if nicRef.Type != "" {
		return networkMap.FindAllNetworksByType(nicRef.Type)
	}
	if nicRef.Name != "" {
		return networkMap.FindAllNetworksByNameAndNamespace(nicRef.Namespace, nicRef.Name)
	}
	return nil
}

// NADPool tracks NAD assignments within a single VM to ensure no NAD
// is used twice. Create one per VM via NewNADPool().
type NADPool struct {
	used map[string]bool
}

// NewNADPool creates a NADPool for tracking NAD assignments on one VM.
func NewNADPool() *NADPool {
	return &NADPool{
		used: make(map[string]bool),
	}
}

func nadKey(dest api.DestinationNetwork) string {
	if dest.Namespace == "" {
		return dest.Name
	}
	return dest.Namespace + "/" + dest.Name
}

// Allocate picks the first Multus NAD not yet used on this VM.
// pairsForSource are pre-filtered by source network (matched by ID or
// name), so every pair shares the same source. Only pass Multus pairs;
// for mixed-type routing use AllocateNetwork.
func (p *NADPool) Allocate(pairsForSource []api.NetworkPair) (api.NetworkPair, bool) {
	if len(pairsForSource) == 0 {
		return api.NetworkPair{}, false
	}

	if len(pairsForSource) == 1 {
		return pairsForSource[0], true
	}

	for _, pair := range pairsForSource {
		key := nadKey(pair.Destination)
		if !p.used[key] {
			p.used[key] = true
			return pair, true
		}
	}
	return api.NetworkPair{}, false
}

// AllocateNetwork picks a destination for one NIC from pre-filtered
// pairs (already matched to the NIC's source network by ID or name).
// Non-Multus destinations pass through directly; Multus destinations go
// through the NADPool for deduplication.
func AllocateNetwork(pool *NADPool, pairsForSource []api.NetworkPair) (api.NetworkPair, bool) {
	var nadPairs []api.NetworkPair
	for _, pair := range pairsForSource {
		if pair.Destination.Type != Multus {
			return pair, true
		}
		nadPairs = append(nadPairs, pair)
	}
	return pool.Allocate(nadPairs)
}

// ValidatePodNetworkDuplicates reports whether more than one NIC resolves
// to the pod network. Duplicate Multus NAD assignments are allowed.
func ValidatePodNetworkDuplicates(nicRefs []ref.Ref, networkMap *api.NetworkMap) bool {
	if networkMap == nil {
		return false
	}

	pool := NewNADPool()
	podCount := 0
	for _, nicRef := range nicRefs {
		pairs := FindAllMappingsForNICRef(nicRef, networkMap)
		if len(pairs) == 0 {
			continue
		}
		pair, ok := AllocateNetwork(pool, pairs)
		if !ok {
			continue
		}
		if pair.Destination.Type == Pod {
			podCount++
		}
	}
	return podCount > 1
}
