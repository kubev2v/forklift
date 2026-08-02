package vmware

import (
	"fmt"
	"strings"

	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/storage"
	"github.com/vmware/govmomi/vim25/types"
)

func normalizePort(port string) string {
	return strings.ToLower(strings.ReplaceAll(port, ":", ""))
}

// selectAdaptersForArray returns the host adapters that can reach the
// destination array. PowerFlex/scini is checked first and never goes through
// target-port matching: its FC-shaped adapter identity (see client.go) is a
// driver-invented value with no real array-side counterpart to compare
// against. Otherwise, it matches each local HBA's ScsiTopology-visible SAN
// targets — independent of whether any LUN is mapped through them, since FC
// port-login / iSCSI session establishment happen before any LUN masking —
// against arrayIdentifier's own reported target ports.
func selectAdaptersForArray(hbaByKey map[string]HostAdapter, topology *types.HostScsiTopology, arrayIdentifier storage.ArrayIdentifier, sciniGuid string) ([]HostAdapter, error) {
	if scini, ok := arrayIdentifier.(storage.SciniAware); ok && scini.SciniRequired() {
		for _, adapter := range hbaByKey {
			if adapter.Driver == "scini" {
				if sciniGuid != "" {
					adapter.Id = sciniGuid
				}
				return []HostAdapter{adapter}, nil
			}
		}
		return nil, fmt.Errorf("destination requires PowerFlex/scini but host has no scini adapter")
	}

	targetPorts, err := arrayIdentifier.TargetPorts()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch destination array target ports: %w", err)
	}
	wanted := make(map[string]bool, len(targetPorts))
	for _, p := range targetPorts {
		wanted[normalizePort(p)] = true
	}

	var result []HostAdapter
	for _, iface := range topology.Adapter {
		adapter, ok := hbaByKey[iface.Adapter]
		if !ok {
			continue
		}
		for _, target := range iface.Target {
			var identity string
			switch t := target.Transport.(type) {
			case *types.HostFibreChannelTargetTransport:
				identity = fmt.Sprintf("fc.%016x", uint64(t.PortWorldWideName)) // port WWN alone, not the initiator's WWNN:WWPN pair
			case *types.HostInternetScsiTargetTransport:
				identity = t.IScsiName
			default:
				continue
			}
			if wanted[normalizePort(identity)] {
				result = append(result, adapter)
				break
			}
		}
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no host adapter is zoned to reach the destination array (checked %d target port(s))", len(targetPorts))
	}
	return result, nil
}
