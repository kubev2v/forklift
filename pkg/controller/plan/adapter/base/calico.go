package base

import (
	"encoding/json"
	"fmt"
	"strconv"

	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// Per-interface (Calico secondary-NAD path) annotation keys. Scoped by
	// the VMI network name (net-0, net-1, or template-derived). At runtime
	// KubeVirt names the pod-side interface with its hashed scheme, but
	// Calico's CNI reverse-maps that back to the VMI network name for
	// virt-launcher pods and keys per-interface annotations by it — so the
	// VMI network name is the correct key here, not the hashed name.
	CalicoAnnHwAddrFmt = "cni.projectcalico.org/%s.hwAddr"
	CalicoAnnIPsFmt    = "cni.projectcalico.org/%s.ipAddrs"

	// Unscoped (Calico primary-NIC path) annotation keys. Applied to the
	// VM's pod template once, addressing the pod's primary interface
	// (Calico's default). The Networks annotation pulls in an L2 attach via
	// a named projectcalico.org/v3 Network CR; absence means default L3
	// IPAM. The Vlan annotation selects the 802.1Q VLAN within that
	// Network; when omitted, Calico defaults to the Network's sole VLAN
	// entry (multi-VLAN Networks require an explicit selection — enforced
	// at Plan validation).
	CalicoAnnPrimaryHwAddr  = "cni.projectcalico.org/hwAddr"
	CalicoAnnPrimaryIPs     = "cni.projectcalico.org/ipAddrs"
	CalicoAnnPrimaryNetwork = "cni.projectcalico.org/networks"
	CalicoAnnPrimaryVlan    = "cni.projectcalico.org/vlan"

	// AnnAllowPodBridgeNetworkLiveMigration is KubeVirt's opt-in for live
	// migration of VMs whose pod-network interface uses Bridge binding.
	// Calico-primary VMs are bridge-bound, so without this annotation they
	// cannot live-migrate — and Calico's IP persistence across live
	// migration is a headline capability of the L2 feature.
	AnnAllowPodBridgeNetworkLiveMigration = "kubevirt.io/allow-pod-bridge-network-live-migration"
)

// SetCalicoMAC writes the cni.projectcalico.org/<ifname>.hwAddr annotation
// onto m. Lazy-inits Annotations when nil.
func SetCalicoMAC(m *meta.ObjectMeta, ifname, mac string) {
	if m.Annotations == nil {
		m.Annotations = map[string]string{}
	}
	m.Annotations[fmt.Sprintf(CalicoAnnHwAddrFmt, ifname)] = mac
}

// SetCalicoStaticIPs JSON-marshals ips and writes the
// cni.projectcalico.org/<ifname>.ipAddrs annotation. No-op when ips is empty.
// Lazy-inits Annotations when nil.
func SetCalicoStaticIPs(m *meta.ObjectMeta, ifname string, ips []string) error {
	if len(ips) == 0 {
		return nil
	}
	encoded, err := json.Marshal(ips)
	if err != nil {
		return err
	}
	if m.Annotations == nil {
		m.Annotations = map[string]string{}
	}
	m.Annotations[fmt.Sprintf(CalicoAnnIPsFmt, ifname)] = string(encoded)
	return nil
}

// CalicoPrimaryParams collects the Calico-primary annotation inputs so a
// single StampCalicoPrimary call can write all of them at once.
type CalicoPrimaryParams struct {
	// MAC is the source NIC's MAC address. Empty → no annotation.
	MAC string
	// IPs are the IPv4 addresses of the source NIC (passed through only
	// when Plan.Spec.PreserveStaticIPs is true). Empty/nil → no annotation.
	IPs []string
	// Network is the name of a projectcalico.org/v3 Network CR for L2
	// attach. Empty → no annotation; pod uses Calico's default L3 IPAM.
	Network string
	// Vlan is the 802.1Q VLAN ID within the named Network. Zero → no
	// annotation; Calico defaults to the Network's sole VLAN entry (the
	// Plan validator rejects the implicit form on multi-VLAN Networks).
	Vlan uint16
}

// SetCalicoPrimaryMAC writes the unscoped cni.projectcalico.org/hwAddr
// annotation. No-op when mac is empty. Lazy-inits Annotations when nil.
func SetCalicoPrimaryMAC(m *meta.ObjectMeta, mac string) {
	if mac == "" {
		return
	}
	if m.Annotations == nil {
		m.Annotations = map[string]string{}
	}
	m.Annotations[CalicoAnnPrimaryHwAddr] = mac
}

// SetCalicoPrimaryStaticIPs JSON-marshals ips and writes the unscoped
// cni.projectcalico.org/ipAddrs annotation. No-op when ips is empty.
// Lazy-inits Annotations when nil.
func SetCalicoPrimaryStaticIPs(m *meta.ObjectMeta, ips []string) error {
	if len(ips) == 0 {
		return nil
	}
	encoded, err := json.Marshal(ips)
	if err != nil {
		return err
	}
	if m.Annotations == nil {
		m.Annotations = map[string]string{}
	}
	m.Annotations[CalicoAnnPrimaryIPs] = string(encoded)
	return nil
}

// SetCalicoPrimaryNetwork writes the cni.projectcalico.org/networks
// annotation with the named Calico Network CR. No-op when network is empty
// (default L3 IPAM). Lazy-inits Annotations when nil.
func SetCalicoPrimaryNetwork(m *meta.ObjectMeta, network string) {
	if network == "" {
		return
	}
	if m.Annotations == nil {
		m.Annotations = map[string]string{}
	}
	m.Annotations[CalicoAnnPrimaryNetwork] = network
}

// SetCalicoPrimaryVlan writes the cni.projectcalico.org/vlan annotation as a
// decimal 802.1Q VLAN ID. No-op when vlan is zero (Calico defaults to the
// named Network's sole VLAN entry). Lazy-inits Annotations when nil.
func SetCalicoPrimaryVlan(m *meta.ObjectMeta, vlan uint16) {
	if vlan == 0 {
		return
	}
	if m.Annotations == nil {
		m.Annotations = map[string]string{}
	}
	m.Annotations[CalicoAnnPrimaryVlan] = strconv.Itoa(int(vlan))
}

// StampCalicoPrimary applies the full Calico-primary annotation set to m in
// one call. Each individual annotation is no-op on its empty input, so
// callers can pass zero-value fields freely.
func StampCalicoPrimary(m *meta.ObjectMeta, p CalicoPrimaryParams) error {
	SetCalicoPrimaryMAC(m, p.MAC)
	if err := SetCalicoPrimaryStaticIPs(m, p.IPs); err != nil {
		return err
	}
	SetCalicoPrimaryNetwork(m, p.Network)
	SetCalicoPrimaryVlan(m, p.Vlan)
	return nil
}
