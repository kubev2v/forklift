package base

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
)

// Network types
const (
	Pod     = "pod"
	Multus  = "multus"
	Ignored = "ignored"
)

// NetworkMappingMatcher is a callback function that providers implement
// to handle their specific network matching logic.
// It should return true if the mapping applies to the VM's networks.
// The vm parameter is the VM object retrieved by the provider.
type NetworkMappingMatcher func(vm interface{}, mapping *api.NetworkPair) (bool, error)

// VMRetriever is a callback function that providers implement
// to retrieve VM data in their provider-specific way.
type VMRetriever func(vmRef ref.Ref) (vm interface{}, err error)

// ValidateNetworkMapping provides shared network mapping validation logic
// that all providers can use. It validates that:
// 1. Only one Pod network can be mapped per VM
// 2. Multus networks must have unique destination names
// 3. Other network types (like "ignored") have no restrictions
//
// The retriever function handles provider-specific VM retrieval.
// The matcher function allows each provider to implement their specific
// network matching logic while sharing the common validation rules.
func ValidateNetworkMapping(ctx *plancontext.Context, vmRef ref.Ref, retriever VMRetriever, matcher NetworkMappingMatcher) (ok bool, err error) {
	if ctx.Plan.Referenced.Map.Network == nil {
		ok = true
		return
	}

	// Retrieve VM using provider-specific logic
	vm, err := retriever(vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}

	mapping := ctx.Plan.Referenced.Map.Network.Spec.Map
	podMapped := 0
	multusNames := make(map[string]int) // Track Multus destination names

	for i := range mapping {
		mapped := &mapping[i]

		// Use provider-specific matching logic
		matches, fErr := matcher(vm, mapped)
		if fErr != nil {
			err = fErr
			return
		}

		if matches {
			switch mapped.Destination.Type {
			case Pod:
				podMapped++
			case Multus:
				// For Multus, track the destination name to ensure uniqueness
				multusNames[mapped.Destination.Name]++
			}
		}
	}

	// Check Pod validation: only one Pod network allowed
	if podMapped > 1 {
		ok = false
		return
	}

	// Check Multus validation: each Multus destination name must be unique
	for _, count := range multusNames {
		if count > 1 {
			ok = false
			return
		}
	}

	ok = true
	return
}
