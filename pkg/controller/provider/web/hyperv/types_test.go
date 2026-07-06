package hyperv

import (
	"strings"
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/hyperv"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
)

func testProvider() *api.Provider {
	pt := api.HyperV
	return &api.Provider{
		ObjectMeta: v1.ObjectMeta{
			Name: "test-provider",
			UID:  k8stypes.UID("provider-uid-123"),
		},
		Spec: api.ProviderSpec{Type: &pt},
	}
}

func TestCluster_With(t *testing.T) {
	m := &model.Cluster{
		Base:   model.Base{ID: "c1", Name: "cluster01", Revision: 5},
		Domain: "lab.local",
		Nodes: []model.Ref{
			{Kind: model.HostKind, ID: "node-a"},
			{Kind: model.HostKind, ID: "node-b"},
		},
	}
	r := &Cluster{}
	r.With(m)

	if r.ID != "c1" {
		t.Errorf("ID = %q, want 'c1'", r.ID)
	}
	if r.Name != "cluster01" {
		t.Errorf("Name = %q, want 'cluster01'", r.Name)
	}
	if r.Domain != "lab.local" {
		t.Errorf("Domain = %q, want 'lab.local'", r.Domain)
	}
	if len(r.Nodes) != 2 {
		t.Fatalf("Nodes count = %d, want 2", len(r.Nodes))
	}
}

func TestCluster_Link(t *testing.T) {
	r := &Cluster{}
	r.ID = "c1"
	r.Link(testProvider())

	if !strings.Contains(r.SelfLink, "provider-uid-123") {
		t.Errorf("SelfLink missing provider UID: %s", r.SelfLink)
	}
	if !strings.Contains(r.SelfLink, "c1") {
		t.Errorf("SelfLink missing cluster ID: %s", r.SelfLink)
	}
}

func TestCluster_Content(t *testing.T) {
	r := &Cluster{Domain: "lab.local"}
	r.ID = "c1"

	summary := r.Content(0)
	if _, ok := summary.(Resource); !ok {
		t.Error("Content(0) should return Resource (summary)")
	}
	full := r.Content(1)
	if _, ok := full.(*Cluster); !ok {
		t.Error("Content(1) should return *Cluster (full)")
	}
}

func TestHost_With(t *testing.T) {
	m := &model.Host{
		Base:        model.Base{ID: "h1", Name: "node-a", Revision: 3},
		Cluster:     "cluster01",
		State:       "Up",
		CpuSockets:  2,
		CpuCores:    16,
		MemoryBytes: 34359738368,
		Networks:    []model.Ref{{Kind: model.NetKind, ID: "net-1"}},
	}
	r := &Host{}
	r.With(m)

	if r.ID != "h1" {
		t.Errorf("ID = %q, want 'h1'", r.ID)
	}
	if r.Cluster != "cluster01" {
		t.Errorf("Cluster = %q, want 'cluster01'", r.Cluster)
	}
	if r.State != "Up" {
		t.Errorf("State = %q, want 'Up'", r.State)
	}
	if r.CpuSockets != 2 {
		t.Errorf("CpuSockets = %d, want 2", r.CpuSockets)
	}
	if r.CpuCores != 16 {
		t.Errorf("CpuCores = %d, want 16", r.CpuCores)
	}
	if r.MemoryBytes != 34359738368 {
		t.Errorf("MemoryBytes = %d, want 34359738368", r.MemoryBytes)
	}
	if len(r.Networks) != 1 {
		t.Fatalf("Networks count = %d, want 1", len(r.Networks))
	}
}

func TestHost_Link(t *testing.T) {
	r := &Host{}
	r.ID = "h1"
	r.Link(testProvider())

	if !strings.Contains(r.SelfLink, "provider-uid-123") {
		t.Errorf("SelfLink missing provider UID: %s", r.SelfLink)
	}
	if !strings.Contains(r.SelfLink, "h1") {
		t.Errorf("SelfLink missing host ID: %s", r.SelfLink)
	}
}

func TestResolver_Path_Cluster(t *testing.T) {
	r := &Resolver{Provider: testProvider()}
	path, err := r.Path(&Cluster{}, "c1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(path, "clusters/c1") {
		t.Errorf("Path = %q, expected to contain 'clusters/c1'", path)
	}
	if !strings.Contains(path, "provider-uid-123") {
		t.Errorf("Path = %q, expected to contain provider UID", path)
	}
}

func TestResolver_Path_Host(t *testing.T) {
	r := &Resolver{Provider: testProvider()}
	path, err := r.Path(&Host{}, "h1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(path, "hosts/h1") {
		t.Errorf("Path = %q, expected to contain 'hosts/h1'", path)
	}
}

func TestResolver_Path_VM(t *testing.T) {
	r := &Resolver{Provider: testProvider()}
	path, err := r.Path(&VM{}, "vm1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(path, "vms/vm1") {
		t.Errorf("Path = %q, expected to contain 'vms/vm1'", path)
	}
}

func TestResolver_Path_Unknown(t *testing.T) {
	r := &Resolver{Provider: testProvider()}
	_, err := r.Path("unsupported", "id")
	if err == nil {
		t.Error("Expected error for unsupported resource type")
	}
}
