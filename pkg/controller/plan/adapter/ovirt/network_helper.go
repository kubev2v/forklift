package ovirt

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planbase "github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	web "github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/ovirt"
)

type resolvedNIC struct {
	NIC     model.XNIC
	Mapping *api.NetworkPair
}

func buildOvirtNICResolver(
	nics []model.XNIC,
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
		if err := inventory.Find(network, entry.Source.Ref); err != nil {
			return nil, nil, err
		}
		pairsBySource[network.ID] = append(pairsBySource[network.ID], *entry)
	}
	nicKeys := make([]string, len(nics))
	for i, nic := range nics {
		nicKeys[i] = nic.Profile.Network
	}
	return nicKeys, pairsBySource, nil
}

// resolveNICMappings matches VM NICs to network map entries using
// AllocateNetwork to ensure each NIC gets a distinct NAD when multiple
// map entries share the same source.
func resolveNICMappings(
	nics []model.XNIC,
	netMap []api.NetworkPair,
	inventory web.Client,
) ([]resolvedNIC, error) {
	nicKeys, pairsBySource, err := buildOvirtNICResolver(nics, netMap, inventory)
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
