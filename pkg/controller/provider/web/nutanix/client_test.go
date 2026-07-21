package nutanix

import (
	"errors"
	"strings"
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func newTestResolver() *Resolver {
	return &Resolver{
		Provider: &api.Provider{ObjectMeta: meta.ObjectMeta{UID: types.UID("provider-1")}},
	}
}

// TestResolverPath_KnownResources verifies that every resource type
// registered with the Resolver produces a path ending in the given ID, with
// every route placeholder substituted.
func TestResolverPath_KnownResources(t *testing.T) {
	r := newTestResolver()

	tests := []struct {
		name     string
		resource interface{}
	}{
		{"Provider", &Provider{}},
		{"Cluster", &Cluster{}},
		{"Host", &Host{}},
		{"Network", &Network{}},
		{"StorageContainer", &StorageContainer{}},
		{"Image", &Image{}},
		{"VM", &VM{}},
		{"Workload", &Workload{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := r.Path(tt.resource, "obj-1")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.HasSuffix(path, "obj-1") {
				t.Errorf("expected path to end with the resource ID, got %q", path)
			}
			if strings.Contains(path, ":") {
				t.Errorf("expected every route placeholder to be substituted, got %q", path)
			}
		})
	}
}

// TestResolverPath_UnknownResource verifies that an unrecognized resource
// type returns a ResourceNotResolvedError instead of an empty path.
func TestResolverPath_UnknownResource(t *testing.T) {
	r := newTestResolver()

	type unknownResource struct{}

	_, err := r.Path(&unknownResource{}, "obj-1")
	if err == nil {
		t.Fatal("expected an error for an unregistered resource type")
	}
	var notResolved ResourceNotResolvedError
	if !errors.As(err, &notResolved) {
		t.Errorf("expected ResourceNotResolvedError, got %T: %v", err, err)
	}
}
