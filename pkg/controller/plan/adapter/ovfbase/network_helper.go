package ovfbase

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planbase "github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	ovfmodel "github.com/kubev2v/forklift/pkg/controller/provider/model/ovf"
	web "github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/ova"
)

type resolvedNIC struct {
	NIC     ovfmodel.NIC
	Mapping *api.NetworkPair
}

func buildOvfNICResolver(
	nics []ovfmodel.NIC,
	netMap []api.NetworkPair,
	inventory web.Client,
) ([]string, map[string][]api.NetworkPair, error) {
	pairsBySource := map[string][]api.NetworkPair{}
	for i := range netMap {
		entry := &netMap[i]
		if entry.Destination.Type == planbase.Ignored {
			continue
		}
		network := &model.Network{}
		if err := inventory.Find(network, entry.Source); err != nil {
			return nil, nil, err
		}
		pairsBySource[network.Name] = append(pairsBySource[network.Name], *entry)
	}
	nicKeys := make([]string, len(nics))
	for i, nic := range nics {
		nicKeys[i] = nic.Network
	}
	return nicKeys, pairsBySource, nil
}

// resolveNICMappings matches VM NICs to network map entries using
// AllocateNetwork to ensure each NIC gets a distinct NAD when multiple
// map entries share the same source.
func resolveNICMappings(
	nics []ovfmodel.NIC,
	netMap []api.NetworkPair,
	inventory web.Client,
) ([]resolvedNIC, error) {
	nicKeys, pairsBySource, err := buildOvfNICResolver(nics, netMap, inventory)
	if err != nil {
		return nil, err
	}
	pool := planbase.NewNADPool()
	var result []resolvedNIC
	for i, nic := range nics {
		pair, allocated := planbase.AllocateNetwork(pool, pairsBySource[nicKeys[i]])
		if !allocated {
			continue
		}
		result = append(result, resolvedNIC{NIC: nic, Mapping: &pair})
	}
	return result, nil
}
