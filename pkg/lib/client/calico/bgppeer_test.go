package calico

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

// makeBGPPeer builds an unstructured projectcalico.org/v3 BGPPeer. An empty
// network leaves spec.network unset — the shape of a peer that predates the
// field or peers with the whole cluster rather than a Network.
func makeBGPPeer(name, network string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(BGPPeerGVK)
	u.SetName(name)
	_ = unstructured.SetNestedField(u.Object, int64(64512), "spec", "asNumber")
	if network != "" {
		_ = unstructured.SetNestedField(u.Object, network, "spec", "network")
	}
	return u
}

func newFakeClientWithBGPPeers(objs ...runtime.Object) client.Client {
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(BGPPeerGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(BGPPeerGVK.GroupVersion().WithKind("BGPPeerList"), &unstructured.UnstructuredList{})
	return fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).Build()
}

func TestListBGPPeerNetworks_Empty(t *testing.T) {
	c := newFakeClientWithBGPPeers()
	got, err := ListBGPPeerNetworks(context.Background(), c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %v, want empty set", got)
	}
}

func TestListBGPPeerNetworks_Mixed(t *testing.T) {
	// Peers with spec.network contribute their Network name; peers without
	// the field are skipped. Two peers naming the same Network dedupe.
	c := newFakeClientWithBGPPeers(
		makeBGPPeer("vrf-red-peer", "vrf-red"),
		makeBGPPeer("vrf-red-peer-2", "vrf-red"),
		makeBGPPeer("vrf-blue-peer", "vrf-blue"),
		makeBGPPeer("global-peer", ""),
	)
	got, err := ListBGPPeerNetworks(context.Background(), c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]bool{"vrf-red": true, "vrf-blue": true}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestListBGPPeerNetworks_KindAbsent(t *testing.T) {
	// The API server does not know the BGPPeer kind (older install, no
	// CRD). Reported as the empty set — no peer bound anywhere — not as an
	// error.
	c := fake.NewClientBuilder().
		WithInterceptorFuncs(interceptor.Funcs{
			List: func(ctx context.Context, _ client.WithWatch, _ client.ObjectList, _ ...client.ListOption) error {
				return &meta.NoKindMatchError{GroupKind: BGPPeerGVK.GroupKind()}
			},
		}).Build()
	got, err := ListBGPPeerNetworks(context.Background(), c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %v, want empty set", got)
	}
}

func TestListBGPPeerNetworks_ListError(t *testing.T) {
	// Any list failure other than kind-absent must propagate.
	boom := errors.New("connection refused")
	c := fake.NewClientBuilder().
		WithInterceptorFuncs(interceptor.Funcs{
			List: func(ctx context.Context, _ client.WithWatch, _ client.ObjectList, _ ...client.ListOption) error {
				return boom
			},
		}).Build()
	if _, err := ListBGPPeerNetworks(context.Background(), c); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want %v", err, boom)
	}
}

func TestListBGPPeerNetworks_BadNetworkType(t *testing.T) {
	peer := makeBGPPeer("bad-peer", "")
	_ = unstructured.SetNestedField(peer.Object, int64(7), "spec", "network")
	c := newFakeClientWithBGPPeers(peer)
	if _, err := ListBGPPeerNetworks(context.Background(), c); err == nil {
		t.Fatal("expected error, got nil")
	}
}
