package nutanix

import (
	"testing"

	libclient "github.com/kubev2v/forklift/pkg/lib/client/nutanix"
)

func TestCoalesce(t *testing.T) {
	if result := libclient.Coalesce("", "value-b", "value-c"); result != "value-b" {
		t.Errorf("expected first non-empty value 'value-b', got %q", result)
	}
	if result := libclient.Coalesce(""); result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestCoalesceInt(t *testing.T) {
	if result := coalesceInt(0, 7); result != 7 {
		t.Errorf("expected first non-zero value 7, got %d", result)
	}
	if result := coalesceInt(0); result != 0 {
		t.Errorf("expected fallback to first key's value 0, got %d", result)
	}
}

func TestCoalesceInt64(t *testing.T) {
	if result := coalesceInt64(0, 42); result != 42 {
		t.Errorf("expected first non-zero value 42, got %d", result)
	}
	if result := coalesceInt64(0); result != 0 {
		t.Errorf("expected fallback to first key's value 0, got %d", result)
	}
}

func TestCoalesceBool(t *testing.T) {
	if !coalesceBool(false, true) {
		t.Error("expected coalesceBool to find the first true value")
	}
	if coalesceBool(false) {
		t.Error("expected false when no candidate is true")
	}
}

func TestParseNumericString(t *testing.T) {
	tests := []struct {
		name      string
		input     any
		expected  int64
		expectOK  bool
	}{
		{"numeric string", "12345", 12345, true},
		{"non-numeric string", "not-a-number", 0, false},
		{"int", 42, 42, true},
		{"int64", int64(99), 99, true},
		{"float64", float64(7), 7, true},
		{"unsupported type", true, 0, false},
		{"missing value", nil, 0, false},
		{"zero string", "0", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := libclient.ParseNumericString(tt.input)
			if result != tt.expected || ok != tt.expectOK {
				t.Errorf("expected (%d, %v), got (%d, %v)", tt.expected, tt.expectOK, result, ok)
			}
		})
	}
}

func TestFilterByMatch_EmptyClusterUUID(t *testing.T) {
	entities := []clusterEntity{
		{Metadata: libclient.Metadata{UUID: "cluster-a"}},
		{Metadata: libclient.Metadata{UUID: "cluster-b"}},
	}

	filtered := filterByMatch(entities, "", func(entity clusterEntity) string {
		return entity.Metadata.UUID
	})
	if len(filtered) != len(entities) {
		t.Errorf("expected every entity to be returned unfiltered when clusterUUID is empty, got %d", len(filtered))
	}
}

func TestFilterByMatch_Matches(t *testing.T) {
	entities := []clusterEntity{
		{Metadata: libclient.Metadata{UUID: "cluster-a"}},
		{Metadata: libclient.Metadata{UUID: "cluster-b"}},
	}

	filtered := filterByMatch(entities, "cluster-a", func(entity clusterEntity) string {
		return entity.Metadata.UUID
	})
	if len(filtered) != 1 {
		t.Fatalf("expected 1 matching entity, got %d", len(filtered))
	}
	if filtered[0].Metadata.UUID != "cluster-a" {
		t.Errorf("expected cluster-a to survive, got %s", filtered[0].Metadata.UUID)
	}
}

func TestFilterByMatch_FallbackClusterRef(t *testing.T) {
	entities := []hostEntity{
		{Spec: struct {
			ClusterReference libclient.Ref `json:"cluster_reference"`
			Name             string        `json:"name"`
		}{ClusterReference: libclient.Ref{UUID: "cluster-a"}}},
		{Status: struct {
			ClusterReference libclient.Ref `json:"cluster_reference"`
			Name             string        `json:"name"`
			State            string        `json:"state"`
			Resources        hostResources `json:"resources"`
		}{ClusterReference: libclient.Ref{UUID: "cluster-a"}}},
		{Spec: struct {
			ClusterReference libclient.Ref `json:"cluster_reference"`
			Name             string        `json:"name"`
		}{ClusterReference: libclient.Ref{UUID: "cluster-b"}}},
	}

	filtered := filterByMatch(entities, "cluster-a", func(entity hostEntity) string {
		return entity.clusterUUID()
	})
	if len(filtered) != 2 {
		t.Fatalf("expected 2 matching entities, got %d", len(filtered))
	}
}

func clusterEntityWithServiceList(uuid string, services ...string) clusterEntity {
	return clusterEntity{
		Metadata: libclient.Metadata{UUID: uuid},
		Status: struct {
			Name      string `json:"name"`
			State     string `json:"state"`
			Resources struct {
				Analysis struct {
					Storage struct {
						TotalCapacityBytes int64 `json:"total_capacity_bytes"`
						UsageBytes         int64 `json:"usage_bytes"`
					} `json:"storage_summary"`
					VMCount int64 `json:"vm_count"`
				} `json:"analysis"`
				Config  clusterConfig `json:"config"`
				Network struct {
					ExternalIP string `json:"external_ip"`
				} `json:"network"`
				Nodes struct {
					HypervisorServerList []struct {
						IP string `json:"ip"`
					} `json:"hypervisor_server_list"`
				} `json:"nodes"`
			} `json:"resources"`
		}{
			Resources: struct {
				Analysis struct {
					Storage struct {
						TotalCapacityBytes int64 `json:"total_capacity_bytes"`
						UsageBytes         int64 `json:"usage_bytes"`
					} `json:"storage_summary"`
					VMCount int64 `json:"vm_count"`
				} `json:"analysis"`
				Config  clusterConfig `json:"config"`
				Network struct {
					ExternalIP string `json:"external_ip"`
				} `json:"network"`
				Nodes struct {
					HypervisorServerList []struct {
						IP string `json:"ip"`
					} `json:"hypervisor_server_list"`
				} `json:"nodes"`
			}{
				Config: clusterConfig{ServiceList: services},
			},
		},
	}
}

func TestIsPrismCentralCluster(t *testing.T) {
	if !isPrismCentralCluster(clusterEntityWithServiceList("pc-cluster", "PRISM_CENTRAL")) {
		t.Error("expected a service_list of [PRISM_CENTRAL] to be detected as Prism Central's pseudo-cluster")
	}
	if isPrismCentralCluster(clusterEntityWithServiceList("real-cluster", "AOS")) {
		t.Error("expected a service_list of [AOS] to not be detected as Prism Central's pseudo-cluster")
	}
	if isPrismCentralCluster(clusterEntity{}) {
		t.Error("expected an entity with no service_list to not be detected as Prism Central's pseudo-cluster")
	}
}

func TestWithoutPrismCentralClusters(t *testing.T) {
	entities := []clusterEntity{
		clusterEntityWithServiceList("real-cluster", "AOS"),
		clusterEntityWithServiceList("pc-cluster", "PRISM_CENTRAL"),
	}

	filtered := withoutPrismCentralClusters(entities)
	if len(filtered) != 1 {
		t.Fatalf("expected 1 cluster to remain, got %d", len(filtered))
	}
	if filtered[0].Metadata.UUID != "real-cluster" {
		t.Errorf("expected the real cluster to remain, got %s", filtered[0].Metadata.UUID)
	}
}

func TestExcludedClusterUUIDs(t *testing.T) {
	entities := []clusterEntity{
		clusterEntityWithServiceList("real-cluster", "AOS"),
		clusterEntityWithServiceList("pc-cluster", "PRISM_CENTRAL"),
	}

	excluded := excludedClusterUUIDs(entities)
	if len(excluded) != 1 || !excluded["pc-cluster"] {
		t.Errorf("expected only pc-cluster to be excluded, got %+v", excluded)
	}
}

func TestExcludeHostsByCluster(t *testing.T) {
	entities := []hostEntity{
		{Spec: struct {
			ClusterReference libclient.Ref `json:"cluster_reference"`
			Name             string        `json:"name"`
		}{ClusterReference: libclient.Ref{UUID: "pc-cluster"}}},
		{Spec: struct {
			ClusterReference libclient.Ref `json:"cluster_reference"`
			Name             string        `json:"name"`
		}{ClusterReference: libclient.Ref{UUID: "real-cluster"}}},
	}

	filtered := excludeHostsByCluster(entities, map[string]bool{"pc-cluster": true})
	if len(filtered) != 1 {
		t.Fatalf("expected 1 entity to remain, got %d", len(filtered))
	}
	if filtered[0].clusterUUID() != "real-cluster" {
		t.Errorf("expected the entity referencing real-cluster to remain, got %s", filtered[0].clusterUUID())
	}

	if filtered := excludeHostsByCluster(entities, map[string]bool{}); len(filtered) != len(entities) {
		t.Errorf("expected no filtering with an empty exclusion set, got %d entities", len(filtered))
	}
}

func TestFilterStorageContainersByCluster(t *testing.T) {
	entities := []storageContainerEntity{
		storageContainerV4Raw{ExtID: "sc-1", Name: "one", ClusterExtID: "cluster-a"}.toEntity(),
		storageContainerV4Raw{ExtID: "sc-2", Name: "two", ClusterExtID: "cluster-b"}.toEntity(),
	}

	filtered := filterStorageContainersByCluster(entities, "cluster-a")
	if len(filtered) != 1 {
		t.Fatalf("expected 1 matching storage container, got %d", len(filtered))
	}
	if filtered[0].Metadata.UUID != "sc-1" {
		t.Errorf("expected sc-1 to survive, got %s", filtered[0].Metadata.UUID)
	}
}
