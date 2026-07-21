package nutanix

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/nutanix"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	core "k8s.io/api/core/v1"
)

// clusterEntity builds a minimal cluster entity with the given UUID/name.
func clusterEntity(uuid, name string) map[string]interface{} {
	return map[string]interface{}{
		"metadata": map[string]interface{}{
			"uuid": uuid,
			"name": name,
		},
		"status": map[string]interface{}{
			"resources": map[string]interface{}{},
		},
	}
}

// hostEntity builds a minimal host entity with the given UUID/name.
func hostEntity(uuid, name string) map[string]interface{} {
	return map[string]interface{}{
		"metadata": map[string]interface{}{
			"uuid": uuid,
			"name": name,
		},
		"status": map[string]interface{}{
			"resources": map[string]interface{}{},
		},
	}
}

// networkEntity builds a minimal subnet/network entity with the given
// UUID/name.
func networkEntity(uuid, name string) map[string]interface{} {
	return map[string]interface{}{
		"metadata": map[string]interface{}{
			"uuid": uuid,
			"name": name,
		},
		"status": map[string]interface{}{
			"resources": map[string]interface{}{},
		},
	}
}

// imageEntity builds a minimal image entity with the given UUID/name.
func imageEntity(uuid, name string) map[string]interface{} {
	return map[string]interface{}{
		"metadata": map[string]interface{}{
			"uuid": uuid,
			"name": name,
		},
		"status": map[string]interface{}{
			"resources": map[string]interface{}{},
		},
	}
}

// vmEntity builds a minimal VM entity with the given UUID/name.
func vmEntity(uuid, name string) map[string]interface{} {
	return map[string]interface{}{
		"metadata": map[string]interface{}{
			"uuid": uuid,
		},
		"spec": map[string]interface{}{
			"name":      name,
			"resources": map[string]interface{}{},
		},
		"status": map[string]interface{}{
			"resources": map[string]interface{}{},
		},
	}
}

// storageContainerRawEntity builds a minimal storage container entity in the
// flat Prism Element (v2) shape, since listStorageContainersElement
// normalizes it via storageContainerEntityFromV2 before applyStorageContainer
// ever sees it -- unlike the other resource types, which are already in
// their final nested metadata/status shape straight off the mock server.
func storageContainerRawEntity(uuid, name string) map[string]interface{} {
	return map[string]interface{}{
		"uuid": uuid,
		"name": name,
	}
}

// newTestCollector builds a Collector backed by a temp on-disk DB and a
// Client pointed at the given mock server.
func newTestCollector(t *testing.T, serverURL string) (*Collector, libmodel.DB) {
	t.Helper()

	dir := t.TempDir()
	db := libmodel.New(filepath.Join(dir, "test.db"), model.All()...)
	if err := db.Open(true); err != nil {
		t.Fatalf("Failed to open test DB: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close(true)
	})

	provider := &api.Provider{
		Spec: api.ProviderSpec{
			// Pin the Prism mode explicitly so every collect method's
			// ensurePrismConfig() call resolves deterministically off the
			// same "return {entities:[...]} for anything" mock server,
			// without an extra auto-detection probe.
			Settings: map[string]string{
				api.NutanixPrismType: api.NutanixPrismElement,
			},
		},
	}
	secret := &core.Secret{
		Data: map[string][]byte{
			"user":     []byte("admin"),
			"password": []byte("password"),
		},
	}

	collector := New(db, provider, secret)
	collector.client.url = serverURL
	collector.client.log = logging.WithName("test")
	collector.client.clientTimeout = 30 * time.Second
	collector.ctx, collector.cancel = context.WithCancel(context.Background())
	t.Cleanup(collector.cancel)

	return collector, db
}

// entityServer serves whatever entities the returned setter last provided,
// as `{"entities": [...]}`, regardless of the request path or method. This
// works for every list-style collect method (clusters/hosts/networks/
// images/vms all page through the same v3 "*/list" POST shape, and, with
// Prism mode pinned to Element in newTestCollector, storage containers hit
// the v2 GET endpoint which returns the same {"entities": [...]} shape).
func entityServer(t *testing.T) (url string, setEntities func([]map[string]interface{}), closeFn func()) {
	t.Helper()

	var entities []map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		body, _ := json.Marshal(map[string]interface{}{
			"entities": entities,
		})
		_, _ = w.Write(body)
	}))

	return server.URL, func(e []map[string]interface{}) { entities = e }, server.Close
}

// TestClusters_DeletesStaleEntries verifies that clusters removed from the
// source are removed from inventory on the following collection cycle.
func TestClusters_DeletesStaleEntries(t *testing.T) {
	url, setEntities, closeServer := entityServer(t)
	defer closeServer()

	collector, db := newTestCollector(t, url)

	// First cycle: two clusters present.
	setEntities([]map[string]interface{}{
		clusterEntity("cluster-1", "Cluster1"),
		clusterEntity("cluster-2", "Cluster2"),
	})
	if err := collector.clusters(); err != nil {
		t.Fatalf("Unexpected error on first collection: %v", err)
	}

	var clusters []model.Cluster
	if err := db.List(&clusters, libmodel.ListOptions{}); err != nil {
		t.Fatalf("Unexpected error listing clusters: %v", err)
	}
	if len(clusters) != 2 {
		t.Fatalf("Expected 2 clusters after first collection, got %d", len(clusters))
	}

	// Second cycle: cluster-2 removed from the source.
	setEntities([]map[string]interface{}{
		clusterEntity("cluster-1", "Cluster1"),
	})
	if err := collector.clusters(); err != nil {
		t.Fatalf("Unexpected error on second collection: %v", err)
	}

	clusters = nil
	if err := db.List(&clusters, libmodel.ListOptions{}); err != nil {
		t.Fatalf("Unexpected error listing clusters: %v", err)
	}
	if len(clusters) != 1 {
		t.Fatalf("Expected 1 cluster after removal, got %d", len(clusters))
	}
	if clusters[0].ID != "cluster-1" {
		t.Fatalf("Expected surviving cluster to be cluster-1, got %s", clusters[0].ID)
	}

	stale := &model.Cluster{Base: model.Base{ID: "cluster-2"}}
	err := db.Get(stale)
	if err == nil {
		t.Fatal("Expected cluster-2 to have been deleted from inventory")
	}
}

// TestClusters_NoChangesWhenSourceUnchanged verifies that a collection cycle
// with an unchanged source set does not delete anything.
func TestClusters_NoChangesWhenSourceUnchanged(t *testing.T) {
	url, setEntities, closeServer := entityServer(t)
	defer closeServer()

	collector, db := newTestCollector(t, url)

	setEntities([]map[string]interface{}{
		clusterEntity("cluster-1", "Cluster1"),
	})
	if err := collector.clusters(); err != nil {
		t.Fatalf("Unexpected error on first collection: %v", err)
	}
	if err := collector.clusters(); err != nil {
		t.Fatalf("Unexpected error on second collection: %v", err)
	}

	var clusters []model.Cluster
	if err := db.List(&clusters, libmodel.ListOptions{}); err != nil {
		t.Fatalf("Unexpected error listing clusters: %v", err)
	}
	if len(clusters) != 1 {
		t.Fatalf("Expected 1 cluster, got %d", len(clusters))
	}
}

// TestStaleIDs exercises the staleIDs helper directly.
func TestStaleIDs(t *testing.T) {
	current := map[string]bool{"a": true, "b": true}
	stale := staleIDs([]string{"a", "b", "c", "d"}, current)
	if len(stale) != 2 {
		t.Fatalf("Expected 2 stale IDs, got %d: %v", len(stale), stale)
	}
	found := map[string]bool{}
	for _, id := range stale {
		found[id] = true
	}
	if !found["c"] || !found["d"] {
		t.Fatalf("Expected stale IDs c and d, got %v", stale)
	}
}

// TestHosts_DeletesStaleEntries verifies that hosts removed from the source
// are removed from inventory on the following collection cycle.
func TestHosts_DeletesStaleEntries(t *testing.T) {
	url, setEntities, closeServer := entityServer(t)
	defer closeServer()

	collector, db := newTestCollector(t, url)

	setEntities([]map[string]interface{}{
		hostEntity("host-1", "Host1"),
		hostEntity("host-2", "Host2"),
	})
	if err := collector.hosts(); err != nil {
		t.Fatalf("Unexpected error on first collection: %v", err)
	}

	setEntities([]map[string]interface{}{
		hostEntity("host-1", "Host1"),
	})
	if err := collector.hosts(); err != nil {
		t.Fatalf("Unexpected error on second collection: %v", err)
	}

	var hosts []model.Host
	if err := db.List(&hosts, libmodel.ListOptions{}); err != nil {
		t.Fatalf("Unexpected error listing hosts: %v", err)
	}
	if len(hosts) != 1 || hosts[0].ID != "host-1" {
		t.Fatalf("Expected only host-1 to survive, got %+v", hosts)
	}

	stale := &model.Host{Base: model.Base{ID: "host-2"}}
	if err := db.Get(stale); err == nil {
		t.Fatal("Expected host-2 to have been deleted from inventory")
	}
}

// TestHosts_NoChangesWhenSourceUnchanged verifies that a collection cycle
// with an unchanged source set does not delete anything.
func TestHosts_NoChangesWhenSourceUnchanged(t *testing.T) {
	url, setEntities, closeServer := entityServer(t)
	defer closeServer()

	collector, db := newTestCollector(t, url)

	setEntities([]map[string]interface{}{
		hostEntity("host-1", "Host1"),
	})
	if err := collector.hosts(); err != nil {
		t.Fatalf("Unexpected error on first collection: %v", err)
	}
	if err := collector.hosts(); err != nil {
		t.Fatalf("Unexpected error on second collection: %v", err)
	}

	var hosts []model.Host
	if err := db.List(&hosts, libmodel.ListOptions{}); err != nil {
		t.Fatalf("Unexpected error listing hosts: %v", err)
	}
	if len(hosts) != 1 {
		t.Fatalf("Expected 1 host, got %d", len(hosts))
	}
}

// TestNetworks_DeletesStaleEntries verifies that networks removed from the
// source are removed from inventory on the following collection cycle.
func TestNetworks_DeletesStaleEntries(t *testing.T) {
	url, setEntities, closeServer := entityServer(t)
	defer closeServer()

	collector, db := newTestCollector(t, url)

	setEntities([]map[string]interface{}{
		networkEntity("net-1", "Net1"),
		networkEntity("net-2", "Net2"),
	})
	if err := collector.networks(); err != nil {
		t.Fatalf("Unexpected error on first collection: %v", err)
	}

	setEntities([]map[string]interface{}{
		networkEntity("net-1", "Net1"),
	})
	if err := collector.networks(); err != nil {
		t.Fatalf("Unexpected error on second collection: %v", err)
	}

	var networks []model.Network
	if err := db.List(&networks, libmodel.ListOptions{}); err != nil {
		t.Fatalf("Unexpected error listing networks: %v", err)
	}
	if len(networks) != 1 || networks[0].ID != "net-1" {
		t.Fatalf("Expected only net-1 to survive, got %+v", networks)
	}

	stale := &model.Network{Base: model.Base{ID: "net-2"}}
	if err := db.Get(stale); err == nil {
		t.Fatal("Expected net-2 to have been deleted from inventory")
	}
}

// TestNetworks_NoChangesWhenSourceUnchanged verifies that a collection cycle
// with an unchanged source set does not delete anything.
func TestNetworks_NoChangesWhenSourceUnchanged(t *testing.T) {
	url, setEntities, closeServer := entityServer(t)
	defer closeServer()

	collector, db := newTestCollector(t, url)

	setEntities([]map[string]interface{}{
		networkEntity("net-1", "Net1"),
	})
	if err := collector.networks(); err != nil {
		t.Fatalf("Unexpected error on first collection: %v", err)
	}
	if err := collector.networks(); err != nil {
		t.Fatalf("Unexpected error on second collection: %v", err)
	}

	var networks []model.Network
	if err := db.List(&networks, libmodel.ListOptions{}); err != nil {
		t.Fatalf("Unexpected error listing networks: %v", err)
	}
	if len(networks) != 1 {
		t.Fatalf("Expected 1 network, got %d", len(networks))
	}
}

// TestStorageContainers_DeletesStaleEntries verifies that storage containers
// removed from the source are removed from inventory on the following
// collection cycle.
func TestStorageContainers_DeletesStaleEntries(t *testing.T) {
	url, setEntities, closeServer := entityServer(t)
	defer closeServer()

	collector, db := newTestCollector(t, url)

	setEntities([]map[string]interface{}{
		storageContainerRawEntity("sc-1", "SC1"),
		storageContainerRawEntity("sc-2", "SC2"),
	})
	if err := collector.storageContainers(); err != nil {
		t.Fatalf("Unexpected error on first collection: %v", err)
	}

	setEntities([]map[string]interface{}{
		storageContainerRawEntity("sc-1", "SC1"),
	})
	if err := collector.storageContainers(); err != nil {
		t.Fatalf("Unexpected error on second collection: %v", err)
	}

	var containers []model.StorageContainer
	if err := db.List(&containers, libmodel.ListOptions{}); err != nil {
		t.Fatalf("Unexpected error listing storage containers: %v", err)
	}
	if len(containers) != 1 || containers[0].ID != "sc-1" {
		t.Fatalf("Expected only sc-1 to survive, got %+v", containers)
	}

	stale := &model.StorageContainer{Base: model.Base{ID: "sc-2"}}
	if err := db.Get(stale); err == nil {
		t.Fatal("Expected sc-2 to have been deleted from inventory")
	}
}

// TestStorageContainers_NoChangesWhenSourceUnchanged verifies that a
// collection cycle with an unchanged source set does not delete anything.
func TestStorageContainers_NoChangesWhenSourceUnchanged(t *testing.T) {
	url, setEntities, closeServer := entityServer(t)
	defer closeServer()

	collector, db := newTestCollector(t, url)

	setEntities([]map[string]interface{}{
		storageContainerRawEntity("sc-1", "SC1"),
	})
	if err := collector.storageContainers(); err != nil {
		t.Fatalf("Unexpected error on first collection: %v", err)
	}
	if err := collector.storageContainers(); err != nil {
		t.Fatalf("Unexpected error on second collection: %v", err)
	}

	var containers []model.StorageContainer
	if err := db.List(&containers, libmodel.ListOptions{}); err != nil {
		t.Fatalf("Unexpected error listing storage containers: %v", err)
	}
	if len(containers) != 1 {
		t.Fatalf("Expected 1 storage container, got %d", len(containers))
	}
}

// TestImages_DeletesStaleEntries verifies that images removed from the
// source are removed from inventory on the following collection cycle.
func TestImages_DeletesStaleEntries(t *testing.T) {
	url, setEntities, closeServer := entityServer(t)
	defer closeServer()

	collector, db := newTestCollector(t, url)

	setEntities([]map[string]interface{}{
		imageEntity("img-1", "Image1"),
		imageEntity("img-2", "Image2"),
	})
	if err := collector.images(); err != nil {
		t.Fatalf("Unexpected error on first collection: %v", err)
	}

	setEntities([]map[string]interface{}{
		imageEntity("img-1", "Image1"),
	})
	if err := collector.images(); err != nil {
		t.Fatalf("Unexpected error on second collection: %v", err)
	}

	var images []model.Image
	if err := db.List(&images, libmodel.ListOptions{}); err != nil {
		t.Fatalf("Unexpected error listing images: %v", err)
	}
	if len(images) != 1 || images[0].ID != "img-1" {
		t.Fatalf("Expected only img-1 to survive, got %+v", images)
	}

	stale := &model.Image{Base: model.Base{ID: "img-2"}}
	if err := db.Get(stale); err == nil {
		t.Fatal("Expected img-2 to have been deleted from inventory")
	}
}

// TestImages_NoChangesWhenSourceUnchanged verifies that a collection cycle
// with an unchanged source set does not delete anything.
func TestImages_NoChangesWhenSourceUnchanged(t *testing.T) {
	url, setEntities, closeServer := entityServer(t)
	defer closeServer()

	collector, db := newTestCollector(t, url)

	setEntities([]map[string]interface{}{
		imageEntity("img-1", "Image1"),
	})
	if err := collector.images(); err != nil {
		t.Fatalf("Unexpected error on first collection: %v", err)
	}
	if err := collector.images(); err != nil {
		t.Fatalf("Unexpected error on second collection: %v", err)
	}

	var images []model.Image
	if err := db.List(&images, libmodel.ListOptions{}); err != nil {
		t.Fatalf("Unexpected error listing images: %v", err)
	}
	if len(images) != 1 {
		t.Fatalf("Expected 1 image, got %d", len(images))
	}
}

// TestVMs_DeletesStaleEntries verifies that VMs removed from the source are
// removed from inventory on the following collection cycle.
func TestVMs_DeletesStaleEntries(t *testing.T) {
	url, setEntities, closeServer := entityServer(t)
	defer closeServer()

	collector, db := newTestCollector(t, url)

	setEntities([]map[string]interface{}{
		vmEntity("vm-1", "VM1"),
		vmEntity("vm-2", "VM2"),
	})
	if err := collector.vms(); err != nil {
		t.Fatalf("Unexpected error on first collection: %v", err)
	}

	setEntities([]map[string]interface{}{
		vmEntity("vm-1", "VM1"),
	})
	if err := collector.vms(); err != nil {
		t.Fatalf("Unexpected error on second collection: %v", err)
	}

	var vms []model.VM
	if err := db.List(&vms, libmodel.ListOptions{}); err != nil {
		t.Fatalf("Unexpected error listing VMs: %v", err)
	}
	if len(vms) != 1 || vms[0].ID != "vm-1" {
		t.Fatalf("Expected only vm-1 to survive, got %+v", vms)
	}

	stale := &model.VM{Base: model.Base{ID: "vm-2"}}
	if err := db.Get(stale); err == nil {
		t.Fatal("Expected vm-2 to have been deleted from inventory")
	}
}

// TestVMs_NoChangesWhenSourceUnchanged verifies that a collection cycle with
// an unchanged source set does not delete anything.
func TestVMs_NoChangesWhenSourceUnchanged(t *testing.T) {
	url, setEntities, closeServer := entityServer(t)
	defer closeServer()

	collector, db := newTestCollector(t, url)

	setEntities([]map[string]interface{}{
		vmEntity("vm-1", "VM1"),
	})
	if err := collector.vms(); err != nil {
		t.Fatalf("Unexpected error on first collection: %v", err)
	}
	if err := collector.vms(); err != nil {
		t.Fatalf("Unexpected error on second collection: %v", err)
	}

	var vms []model.VM
	if err := db.List(&vms, libmodel.ListOptions{}); err != nil {
		t.Fatalf("Unexpected error listing VMs: %v", err)
	}
	if len(vms) != 1 {
		t.Fatalf("Expected 1 VM, got %d", len(vms))
	}
}
