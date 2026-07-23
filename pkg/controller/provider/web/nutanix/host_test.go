package nutanix

import (
	"strings"
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/nutanix"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestHost_With(t *testing.T) {
	m := &model.Host{
		Base:              model.Base{ID: "host-1", Name: "ahv-node-01"},
		HostUUID:          "host-1",
		Cluster:           "cluster-1",
		SerialNumber:      "SN-123",
		BlockModel:        "NX-8035-G8",
		HypervisorType:    "AHV",
		NumVMs:            5,
		State:             "NORMAL",
		HostType:          "HYPER_CONVERGED",
		CPUModel:          "Intel Xeon",
		CPUCapacityHz:     28800000000,
		NumCpuSockets:     2,
		NumCpuCores:       32,
		NumCpuThreads:     64,
		MemoryCapacityMiB: 524288,
		IPMIAddress:       "10.0.0.10",
	}

	r := &Host{}
	r.With(m)

	if r.ID != m.ID || r.Name != m.Name {
		t.Errorf("expected base fields to be copied, got ID=%q Name=%q", r.ID, r.Name)
	}
	if r.HostUUID != m.HostUUID ||
		r.Cluster != m.Cluster ||
		r.SerialNumber != m.SerialNumber ||
		r.BlockModel != m.BlockModel ||
		r.HypervisorType != m.HypervisorType ||
		r.NumVMs != m.NumVMs ||
		r.State != m.State ||
		r.HostType != m.HostType ||
		r.CPUModel != m.CPUModel ||
		r.CPUCapacityHz != m.CPUCapacityHz ||
		r.NumCpuSockets != m.NumCpuSockets ||
		r.NumCpuCores != m.NumCpuCores ||
		r.NumCpuThreads != m.NumCpuThreads ||
		r.MemoryCapacityMiB != m.MemoryCapacityMiB ||
		r.IPMIAddress != m.IPMIAddress {
		t.Errorf("expected With() to copy every model field, got %+v", r)
	}
}

func TestHost_Link(t *testing.T) {
	provider := &api.Provider{ObjectMeta: meta.ObjectMeta{UID: types.UID("provider-1")}}
	r := &Host{}
	r.ID = "host-1"
	r.Link(provider)

	if !strings.Contains(r.SelfLink, "provider-1") {
		t.Errorf("expected SelfLink to contain the provider UID, got %q", r.SelfLink)
	}
	if !strings.HasSuffix(r.SelfLink, "host-1") {
		t.Errorf("expected SelfLink to end with the host ID, got %q", r.SelfLink)
	}
	if strings.Contains(r.SelfLink, ":") {
		t.Errorf("expected SelfLink to have all route placeholders substituted, got %q", r.SelfLink)
	}
}
