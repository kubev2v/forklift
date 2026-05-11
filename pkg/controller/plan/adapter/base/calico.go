package base

import (
	"encoding/json"
	"fmt"

	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	CalicoAnnHwAddrFmt = "cni.projectcalico.org/%s.hwAddr"
	CalicoAnnIPsFmt    = "cni.projectcalico.org/%s.ipAddrs"
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
