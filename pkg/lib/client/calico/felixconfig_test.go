package calico

import (
	"context"
	"errors"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

// makeFelixConfiguration builds an unstructured projectcalico.org/v3
// FelixConfiguration named "default" with the given spec map.
func makeFelixConfiguration(spec map[string]interface{}) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(FelixConfigurationGVK)
	u.SetName(felixConfigurationName)
	if spec != nil {
		_ = unstructured.SetNestedField(u.Object, spec, "spec")
	}
	return u
}

func newFelixFakeClientWith(objs ...runtime.Object) client.Client {
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(FelixConfigurationGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(FelixConfigurationGVK.GroupVersion().WithKind("FelixConfigurationList"), &unstructured.UnstructuredList{})
	return fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).Build()
}

func TestGetBPFEnabled_True(t *testing.T) {
	c := newFelixFakeClientWith(makeFelixConfiguration(map[string]interface{}{
		"bpfEnabled": true,
	}))
	got, err := GetBPFEnabled(context.Background(), c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got {
		t.Error("got false, want true")
	}
}

func TestGetBPFEnabled_False(t *testing.T) {
	c := newFelixFakeClientWith(makeFelixConfiguration(map[string]interface{}{
		"bpfEnabled": false,
	}))
	got, err := GetBPFEnabled(context.Background(), c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got {
		t.Error("got true, want false")
	}
}

func TestGetBPFEnabled_FieldAbsent(t *testing.T) {
	// bpfEnabled defaults to false when the field is not set.
	c := newFelixFakeClientWith(makeFelixConfiguration(map[string]interface{}{
		"logSeverityScreen": "Info",
	}))
	got, err := GetBPFEnabled(context.Background(), c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got {
		t.Error("got true, want false")
	}
}

func TestGetBPFEnabled_DefaultNotFound(t *testing.T) {
	// No "default" FelixConfiguration: Felix runs with defaults, and the
	// default dataplane is not BPF. Per-node configurations don't count.
	perNode := makeFelixConfiguration(map[string]interface{}{"bpfEnabled": true})
	perNode.SetName("node.worker-1")
	c := newFelixFakeClientWith(perNode)
	got, err := GetBPFEnabled(context.Background(), c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got {
		t.Error("got true, want false")
	}
}

func TestGetBPFEnabled_KindAbsent(t *testing.T) {
	// The API server does not know the FelixConfiguration kind. Treated as
	// "not BPF", not as an error.
	c := fake.NewClientBuilder().
		WithInterceptorFuncs(interceptor.Funcs{
			Get: func(ctx context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
				return &meta.NoKindMatchError{GroupKind: FelixConfigurationGVK.GroupKind()}
			},
		}).Build()
	got, err := GetBPFEnabled(context.Background(), c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got {
		t.Error("got true, want false")
	}
}

func TestGetBPFEnabled_BadFieldType(t *testing.T) {
	c := newFelixFakeClientWith(makeFelixConfiguration(map[string]interface{}{
		"bpfEnabled": "yes",
	}))
	if _, err := GetBPFEnabled(context.Background(), c); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetBPFEnabled_TransientError(t *testing.T) {
	// Any GET failure other than NotFound / kind-absent must propagate.
	boom := errors.New("connection refused")
	c := fake.NewClientBuilder().
		WithInterceptorFuncs(interceptor.Funcs{
			Get: func(ctx context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
				return boom
			},
		}).Build()
	if _, err := GetBPFEnabled(context.Background(), c); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want %v", err, boom)
	}
}
