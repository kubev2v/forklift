package base

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
)

type NICRef struct {
	MAC       string
	NetworkID string
}

// NICRefsFrom converts a slice of any NIC type to []NICRef using the provided accessor.
func NICRefsFrom[N any](nics []N, toRef func(N) NICRef) []NICRef {
	refs := make([]NICRef, len(nics))
	for i := range nics {
		refs[i] = toRef(nics[i])
	}
	return refs
}

// ResolveNICModes returns a MAC->mode map based on NetworkMap pairs and NADPool allocation.
func ResolveNICModes(nics []NICRef, networkMap *api.NetworkMap, preserveStaticIPs bool) map[string]string {
	modes := map[string]string{}
	if networkMap == nil {
		// No NetworkMap available — fall back to plan-level preserveStaticIPs for all NICs.
		if preserveStaticIPs {
			for _, nic := range nics {
				modes[nic.MAC] = string(api.NetworkIPModePreserve)
			}
		}
		return modes
	}
	pool := NewNADPool()
	for _, nic := range nics {
		pairs := networkMap.FindAllNetworks(nic.NetworkID)
		if len(pairs) == 0 {
			continue
		}
		pair, allocated := AllocateNetwork(pool, pairs)
		if !allocated {
			// Example: net-1 is mapped to [nad-a, nad-b] but the VM has 3 NICs on net-1.
			// The first two NICs claim nad-a and nad-b, the third NIC has no NAD left.
			// It won't appear in the mode map, so mapMacStaticIps includes it in static
			// IPs by default (backward compat). ValidateNetworkDuplicates warns about this.
			continue
		}
		mode := string(pair.NetworkIPMode)
		if pair.Destination.Type == Ignored {
			mode = string(api.NetworkIPModeNone)
		} else if mode == "" {
			if preserveStaticIPs {
				mode = string(api.NetworkIPModePreserve)
			} else {
				mode = string(api.NetworkIPModeNone)
			}
		}
		modes[nic.MAC] = mode
	}
	return modes
}

func HasPreserveMode(modes map[string]string) bool {
	for _, mode := range modes {
		if mode == string(api.NetworkIPModePreserve) {
			return true
		}
	}
	return false
}

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

// Allocate picks the first Multus NAD not yet used on this VM.
// pairsForSource are pre-filtered by source network (matched by ID or
// name), so every pair shares the same source. Only pass Multus pairs;
// for mixed-type routing use AllocateNetwork.
func (p *NADPool) Allocate(pairsForSource []api.NetworkPair) (api.NetworkPair, bool) {
	for _, pair := range pairsForSource {
		key := pair.Destination.Namespace + "/" + pair.Destination.Name
		if pair.Destination.Namespace == "" {
			key = pair.Destination.Name
		}
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
		// Only Multus NADs need deduplication via the pool.
		if pair.Destination.Type != Multus {
			return pair, true
		}
		nadPairs = append(nadPairs, pair)
	}
	return pool.Allocate(nadPairs)
}

// ValidateNetworkDuplicates checks whether more than one NIC resolves to the
// pod network or more than one NIC resolves to the same Multus NAD name.
// With NAD pool mapping, duplicate NADs are only flagged when the pool for a
// source network is exhausted (NIC count exceeds available NADs).
func ValidateNetworkDuplicates(nicRefs []ref.Ref, networkMap *api.NetworkMap) (foundNadDup bool, foundPodDup bool) {
	if networkMap == nil {
		return
	}

	pool := NewNADPool()
	podCount := 0

	for _, nicRef := range nicRefs {
		pairsForSource := FindAllMappingsForNICRef(nicRef, networkMap)
		pair, allocated := AllocateNetwork(pool, pairsForSource)
		if !allocated {
			if len(pairsForSource) > 0 {
				foundNadDup = true
			}
			continue
		}
		if pair.Destination.Type == Pod {
			podCount++
		}
	}

	foundPodDup = podCount > 1
	return
}
