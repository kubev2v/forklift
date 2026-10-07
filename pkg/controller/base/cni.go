package base

import (
	"encoding/json"
	"net"
	"path"

	k8snet "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
)

// Multus / transfer-network annotation keys (Plan importer pods and CopyApplianceTemplate build pods).
const (
	// AnnLegacyTransferNetwork is the legacy Multus annotation (value=namespace/name).
	// FIXME: phase out in favor of AnnTransferNetwork.
	AnnLegacyTransferNetwork = "v1.multus-cni.io/default-network"
	// AnnTransferNetwork is the modern Multus networks annotation.
	AnnTransferNetwork = "k8s.v1.cni.cncf.io/networks"
	// AnnForkliftNetworkRoute is set on the NAD to specify the default gateway.
	AnnForkliftNetworkRoute = "forklift.konveyor.io/route"
	// AnnForkliftRouteValueNone requests the modern annotation without a default-route.
	AnnForkliftRouteValueNone = "none"
)

// CNINetworkConfig is the CNI network configuration from a NetworkAttachmentDefinition spec.config.
type CNINetworkConfig struct {
	IPAM CNIIPAMConfig `json:"ipam"`
}

// CNIIPAMConfig is the IPAM section of a CNI network configuration.
type CNIIPAMConfig struct {
	Routes []CNIRoute `json:"routes"`
}

// CNIRoute is a single route in CNI IPAM configuration.
type CNIRoute struct {
	Dst string `json:"dst"`
	GW  string `json:"gw"`
}

// GuessTransferNetworkDefaultRoute returns the default gateway for a transfer NAD:
// AnnForkliftNetworkRoute first, else a 0.0.0.0/0 or ::/0 IPAM route.
func GuessTransferNetworkDefaultRoute(netAttachDef *k8snet.NetworkAttachmentDefinition) (route string, found bool) {
	route, found = netAttachDef.Annotations[AnnForkliftNetworkRoute]
	if found {
		return route, true
	}
	if netAttachDef.Spec.Config == "" {
		return "", false
	}
	var config CNINetworkConfig
	if err := json.Unmarshal([]byte(netAttachDef.Spec.Config), &config); err != nil {
		return "", false
	}
	for _, r := range config.IPAM.Routes {
		if r.Dst == "0.0.0.0/0" || r.Dst == "::/0" {
			return r.GW, true
		}
	}
	return "", false
}

// ApplyTransferNetworkAnnotations sets Multus annotations from a NAD (Plan TransferNetwork semantics).
func ApplyTransferNetworkAnnotations(netAttachDef *k8snet.NetworkAttachmentDefinition, annotations map[string]string) (err error) {
	route, found := GuessTransferNetworkDefaultRoute(netAttachDef)
	if found {
		nse := k8snet.NetworkSelectionElement{
			Namespace: netAttachDef.Namespace,
			Name:      netAttachDef.Name,
		}
		if route != AnnForkliftRouteValueNone {
			ip := net.ParseIP(route)
			if ip == nil {
				return liberr.New(
					"Transfer network default route is not a valid IP address.",
					"route", route)
			}
			nse.GatewayRequest = []net.IP{ip}
		}
		var raw []byte
		raw, err = json.Marshal([]k8snet.NetworkSelectionElement{nse})
		if err != nil {
			return liberr.Wrap(err)
		}
		annotations[AnnTransferNetwork] = string(raw)
	} else {
		annotations[AnnLegacyTransferNetwork] = path.Join(netAttachDef.Namespace, netAttachDef.Name)
	}
	return
}
