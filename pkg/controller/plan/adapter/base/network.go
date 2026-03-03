package base

import (
	"fmt"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
)

// Network destination types.
const (
	Pod     = "pod"
	Multus  = "multus"
	Ignored = "ignored"
)

// FindMappingForNICRef returns the NetworkPair whose Source matches the given NIC ref.
func FindMappingForNICRef(nicRef ref.Ref, networkMap *api.NetworkMap) (pair api.NetworkPair, found bool) {
	if networkMap == nil {
		return
	}
	if nicRef.ID != "" {
		return networkMap.FindNetwork(nicRef.ID)
	}
	if nicRef.Type != "" {
		return networkMap.FindNetworkByType(nicRef.Type)
	}
	if nicRef.Name != "" {
		return networkMap.FindNetworkByNameAndNamespace(nicRef.Namespace, nicRef.Name)
	}
	return
}

// ValidateNetworkDuplicates checks whether more than one NIC resolves to the
// pod network or more than one NIC resolves to the same Multus NAD name.
func ValidateNetworkDuplicates(nicRefs []ref.Ref, networkMap *api.NetworkMap) (foundNadDup bool, foundPodDup bool) {
	if networkMap == nil {
		return
	}

	podCount := 0
	nadCount := map[string]int{}

	for _, nicRef := range nicRefs {
		pair, ok := FindMappingForNICRef(nicRef, networkMap)
		if !ok {
			continue
		}
		switch pair.Destination.Type {
		case Pod:
			podCount++
		case Multus:
			nadKey := fmt.Sprintf("%s/%s", pair.Destination.Namespace, pair.Destination.Name)
			nadCount[nadKey]++
		}
	}

	foundPodDup = podCount > 1
	for _, count := range nadCount {
		if count > 1 {
			foundNadDup = true
			break
		}
	}
	return
}
