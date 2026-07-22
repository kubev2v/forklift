package nutanix

import "testing"

func TestFirstString(t *testing.T) {
	values := map[string]interface{}{
		"a": "",
		"b": "value-b",
		"c": "value-c",
	}

	if result := firstString(values, "a", "b", "c"); result != "value-b" {
		t.Errorf("expected first non-empty value 'value-b', got %q", result)
	}
	if result := firstString(values, "missing"); result != "" {
		t.Errorf("expected empty string for missing key, got %q", result)
	}
}

func TestFirstInt(t *testing.T) {
	values := map[string]interface{}{
		"zero":    0,
		"nonzero": 7,
	}

	if result := firstInt(values, "zero", "nonzero"); result != 7 {
		t.Errorf("expected first non-zero value 7, got %d", result)
	}
	// When every candidate is zero/missing, firstInt falls back to
	// re-reading the first key rather than returning a hardcoded 0.
	if result := firstInt(values, "zero", "missing"); result != 0 {
		t.Errorf("expected fallback to first key's value 0, got %d", result)
	}
}

func TestFirstInt64(t *testing.T) {
	values := map[string]interface{}{
		"zero":    int64(0),
		"nonzero": int64(42),
	}

	if result := firstInt64(values, "zero", "nonzero"); result != 42 {
		t.Errorf("expected first non-zero value 42, got %d", result)
	}
	if result := firstInt64(values, "zero", "missing"); result != 0 {
		t.Errorf("expected fallback to first key's value 0, got %d", result)
	}
}

func TestFirstBool(t *testing.T) {
	values := map[string]interface{}{
		"present": true,
		"nested":  map[string]interface{}{"flag": true},
	}

	if result := firstBool(values, "missing", "present"); !result {
		t.Error("expected firstBool to find the first present boolean key")
	}
	if result := firstBool(values, "missing"); result {
		t.Error("expected false when no candidate key is present")
	}
	// firstBool reads keys as top-level map lookups, unlike the other
	// helpers in this file which support dot-separated nested paths.
	if result := firstBool(values, "nested.flag"); result {
		t.Error("expected firstBool to not support dot-separated nested paths")
	}
}

func TestParseNumericString(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected int64
	}{
		{"numeric string", "12345", 12345},
		{"non-numeric string", "not-a-number", 0},
		{"int", 42, 42},
		{"int64", int64(99), 99},
		{"float64", float64(7), 7},
		{"unsupported type", true, 0},
		{"nil", nil, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if result := parseNumericString(tt.input); result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestExtractMapList_MissingKey(t *testing.T) {
	_, err := extractMapList(map[string]interface{}{}, "entities")
	if err == nil {
		t.Fatal("expected an error when the key is missing")
	}
}

func TestExtractMapList_WrongType(t *testing.T) {
	_, err := extractMapList(map[string]interface{}{"entities": "not-a-list"}, "entities")
	if err == nil {
		t.Fatal("expected an error when the value is not a list")
	}
}

func TestExtractMapList_SkipsNonMapEntries(t *testing.T) {
	result := map[string]interface{}{
		"entities": []interface{}{
			map[string]interface{}{"name": "valid"},
			"not-a-map",
			42,
		},
	}

	entities, err := extractMapList(result, "entities")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entities) != 1 {
		t.Fatalf("expected non-map entries to be skipped, got %d entities: %+v", len(entities), entities)
	}
	if entities[0]["name"] != "valid" {
		t.Errorf("expected the surviving entity to be the valid one, got %+v", entities[0])
	}
}

func TestFilterEntitiesByCluster_EmptyClusterUUID(t *testing.T) {
	entities := []map[string]interface{}{
		{"id": "1"},
		{"id": "2"},
	}

	filtered := filterEntitiesByCluster(entities, "", "some.path")
	if len(filtered) != len(entities) {
		t.Errorf("expected every entity to be returned unfiltered when clusterUUID is empty, got %d", len(filtered))
	}
}

func TestFilterEntitiesByCluster_Matches(t *testing.T) {
	entities := []map[string]interface{}{
		{"metadata": map[string]interface{}{"uuid": "cluster-a"}, "id": "1"},
		{"metadata": map[string]interface{}{"uuid": "cluster-b"}, "id": "2"},
	}

	filtered := filterEntitiesByCluster(entities, "cluster-a", "metadata.uuid")
	if len(filtered) != 1 {
		t.Fatalf("expected 1 matching entity, got %d", len(filtered))
	}
	if filtered[0]["id"] != "1" {
		t.Errorf("expected the surviving entity to be id=1, got %+v", filtered[0])
	}
}

// TestFilterEntitiesByCluster_FallbackPath verifies that when the first
// path is absent on an entity, the next path is tried -- needed because
// hosts/subnets carry cluster_reference under spec on some responses and
// under status on others, never nested under status.resources.
func TestFilterEntitiesByCluster_FallbackPath(t *testing.T) {
	entities := []map[string]interface{}{
		{
			"spec": map[string]interface{}{
				"cluster_reference": map[string]interface{}{"uuid": "cluster-a"},
			},
			"id": "1",
		},
		{
			"status": map[string]interface{}{
				"cluster_reference": map[string]interface{}{"uuid": "cluster-a"},
			},
			"id": "2",
		},
		{
			"spec": map[string]interface{}{
				"cluster_reference": map[string]interface{}{"uuid": "cluster-b"},
			},
			"id": "3",
		},
	}

	filtered := filterEntitiesByCluster(entities, "cluster-a",
		"spec.cluster_reference.uuid", "status.cluster_reference.uuid")
	if len(filtered) != 2 {
		t.Fatalf("expected 2 matching entities, got %d: %+v", len(filtered), filtered)
	}
	if filtered[0]["id"] != "1" || filtered[1]["id"] != "2" {
		t.Errorf("expected entities id=1 and id=2 to survive, got %+v", filtered)
	}
}
