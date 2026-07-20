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

	provider := &api.Provider{}
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

// clusterServer serves whatever entities the returned setter last provided
// on any "*/clusters/list" request.
func clusterServer(t *testing.T) (url string, setEntities func([]map[string]interface{}), closeFn func()) {
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
	url, setEntities, closeServer := clusterServer(t)
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
	url, setEntities, closeServer := clusterServer(t)
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
