package nutanix

import (
	"strings"
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/nutanix"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestCluster_With(t *testing.T) {
	m := &model.Cluster{
		Base:          model.Base{ID: "cluster-1", Name: "prod-cluster"},
		ClusterUUID:   "cluster-1",
		Version:       "6.8.2",
		BuildVersion:  "6.8.2-full",
		Timezone:      "America/Los_Angeles",
		ClusterArch:   "X86_64",
		OperationMode: "Normal",
		ExternalIP:    "10.0.0.1",
		NumNodes:      3,
		VMCount:       25,
		TotalCapacity: 1000,
		UsedCapacity:  500,
	}

	r := &Cluster{}
	r.With(m)

	if r.ID != m.ID || r.Name != m.Name {
		t.Errorf("expected base fields to be copied, got ID=%q Name=%q", r.ID, r.Name)
	}
	if r.ClusterUUID != m.ClusterUUID ||
		r.Version != m.Version ||
		r.BuildVersion != m.BuildVersion ||
		r.Timezone != m.Timezone ||
		r.ClusterArch != m.ClusterArch ||
		r.OperationMode != m.OperationMode ||
		r.ExternalIP != m.ExternalIP ||
		r.NumNodes != m.NumNodes ||
		r.VMCount != m.VMCount ||
		r.TotalCapacity != m.TotalCapacity ||
		r.UsedCapacity != m.UsedCapacity {
		t.Errorf("expected With() to copy every model field, got %+v", r)
	}
}

func TestCluster_Link(t *testing.T) {
	provider := &api.Provider{ObjectMeta: meta.ObjectMeta{UID: types.UID("provider-1")}}
	r := &Cluster{}
	r.ID = "cluster-1"
	r.Link(provider)

	if !strings.Contains(r.SelfLink, "provider-1") {
		t.Errorf("expected SelfLink to contain the provider UID, got %q", r.SelfLink)
	}
	if !strings.HasSuffix(r.SelfLink, "cluster-1") {
		t.Errorf("expected SelfLink to end with the cluster ID, got %q", r.SelfLink)
	}
	if strings.Contains(r.SelfLink, ":") {
		t.Errorf("expected SelfLink to have all route placeholders substituted, got %q", r.SelfLink)
	}
}
