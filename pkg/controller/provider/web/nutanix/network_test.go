package nutanix

import (
	"strings"
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/nutanix"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestNetwork_With(t *testing.T) {
	m := &model.Network{
		Base:           model.Base{ID: "net-1", Name: "Production-VLAN-100"},
		NetworkUUID:    "net-1",
		Cluster:        "cluster-1",
		SubnetType:     "VLAN",
		VlanID:         100,
		NetworkAddress: "192.168.100.0",
		PrefixLength:   24,
		DefaultGateway: "192.168.100.1",
		DHCPServerIP:   "192.168.100.2",
		DHCPDomainName: "example.com",
		IPPoolRanges:   "192.168.100.10-192.168.100.100",
	}

	r := &Network{}
	r.With(m)

	if r.ID != m.ID || r.Name != m.Name {
		t.Errorf("expected base fields to be copied, got ID=%q Name=%q", r.ID, r.Name)
	}
	if r.NetworkUUID != m.NetworkUUID ||
		r.Cluster != m.Cluster ||
		r.SubnetType != m.SubnetType ||
		r.VlanID != m.VlanID ||
		r.NetworkAddress != m.NetworkAddress ||
		r.PrefixLength != m.PrefixLength ||
		r.DefaultGateway != m.DefaultGateway ||
		r.DHCPServerIP != m.DHCPServerIP ||
		r.DHCPDomainName != m.DHCPDomainName ||
		r.IPPoolRanges != m.IPPoolRanges {
		t.Errorf("expected With() to copy every model field, got %+v", r)
	}
}

func TestNetwork_Link(t *testing.T) {
	provider := &api.Provider{ObjectMeta: meta.ObjectMeta{UID: types.UID("provider-1")}}
	r := &Network{}
	r.ID = "net-1"
	r.Link(provider)

	if !strings.Contains(r.SelfLink, "provider-1") {
		t.Errorf("expected SelfLink to contain the provider UID, got %q", r.SelfLink)
	}
	if !strings.HasSuffix(r.SelfLink, "net-1") {
		t.Errorf("expected SelfLink to end with the network ID, got %q", r.SelfLink)
	}
	if strings.Contains(r.SelfLink, ":") {
		t.Errorf("expected SelfLink to have all route placeholders substituted, got %q", r.SelfLink)
	}
}
