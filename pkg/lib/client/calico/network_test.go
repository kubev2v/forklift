package calico

import (
	"context"
	"testing"

	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// makeNetwork builds an unstructured projectcalico.org/v3 Network with the
// given name and spec map.
func makeNetwork(name string, spec map[string]interface{}) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(NetworkGVK)
	u.SetName(name)
	if spec != nil {
		_ = unstructured.SetNestedField(u.Object, spec, "spec")
	}
	return u
}

func newFakeClientWith(objs ...runtime.Object) (*fake.ClientBuilder, error) {
	scheme := runtime.NewScheme()
	// Register list-kind so the fake client knows how to map GVK -> Go type
	// for unstructured Network objects. For Gets without a list-kind, the
	// fake client falls back to using the GVK from the object itself.
	scheme.AddKnownTypeWithName(NetworkGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(NetworkGVK.GroupVersion().WithKind("NetworkList"), &unstructured.UnstructuredList{})
	b := fake.NewClientBuilder().WithScheme(scheme)
	for _, o := range objs {
		b = b.WithRuntimeObjects(o)
	}
	return b, nil
}

func TestGetNetwork_NotFound(t *testing.T) {
	cb, err := newFakeClientWith()
	if err != nil {
		t.Fatalf("scheme setup: %v", err)
	}
	c := cb.Build()

	got, err := GetNetwork(context.Background(), c, "missing")
	if got != nil {
		t.Errorf("got non-nil network = %+v, want nil", got)
	}
	if !k8serr.IsNotFound(err) {
		t.Errorf("err = %v, want IsNotFound", err)
	}
}

func TestGetNetwork_NoL2Bridge(t *testing.T) {
	// Network exists but has no l2Bridge spec.
	nw := makeNetwork("flat-net", map[string]interface{}{
		// no l2Bridge key
	})
	cb, _ := newFakeClientWith(nw)
	c := cb.Build()

	got, err := GetNetwork(context.Background(), c, "flat-net")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("got nil network, want non-nil")
	}
	if got.Name != "flat-net" {
		t.Errorf("Name = %q, want flat-net", got.Name)
	}
	if got.L2Bridge != nil {
		t.Errorf("L2Bridge = %+v, want nil", got.L2Bridge)
	}
}

func TestGetNetwork_SingleVLAN(t *testing.T) {
	nw := makeNetwork("vlan100", map[string]interface{}{
		"l2Bridge": map[string]interface{}{
			"vlans": []interface{}{
				map[string]interface{}{
					"vlan": map[string]interface{}{"id": int64(100)},
					"subnets": []interface{}{
						map[string]interface{}{"cidr": "10.100.0.0/24"},
					},
				},
			},
		},
	})
	cb, _ := newFakeClientWith(nw)
	c := cb.Build()

	got, err := GetNetwork(context.Background(), c, "vlan100")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.L2Bridge == nil {
		t.Fatal("L2Bridge = nil, want non-nil")
	}
	if len(got.L2Bridge.VLANs) != 1 {
		t.Fatalf("VLANs len = %d, want 1", len(got.L2Bridge.VLANs))
	}
	v := got.L2Bridge.VLANs[0]
	if v.VID != 100 {
		t.Errorf("VID = %d, want 100", v.VID)
	}
	if len(v.Subnets) != 1 || v.Subnets[0] != "10.100.0.0/24" {
		t.Errorf("Subnets = %v, want [10.100.0.0/24]", v.Subnets)
	}
}

func TestGetNetwork_MultipleVLANs(t *testing.T) {
	nw := makeNetwork("datacenter-vlans", map[string]interface{}{
		"l2Bridge": map[string]interface{}{
			"vlans": []interface{}{
				map[string]interface{}{
					"vlan":    map[string]interface{}{"id": int64(100)},
					"subnets": []interface{}{map[string]interface{}{"cidr": "10.100.0.0/24"}},
				},
				map[string]interface{}{
					"vlan":    map[string]interface{}{"id": int64(200)},
					"subnets": []interface{}{map[string]interface{}{"cidr": "10.200.0.0/24"}},
				},
				map[string]interface{}{
					"vlan": map[string]interface{}{"id": int64(300)},
					"subnets": []interface{}{
						map[string]interface{}{"cidr": "10.30.0.0/16"},
						map[string]interface{}{"cidr": "10.31.0.0/16"},
					},
				},
			},
		},
	})
	cb, _ := newFakeClientWith(nw)
	c := cb.Build()

	got, err := GetNetwork(context.Background(), c, "datacenter-vlans")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.L2Bridge == nil {
		t.Fatal("L2Bridge = nil, want non-nil")
	}
	if len(got.L2Bridge.VLANs) != 3 {
		t.Fatalf("VLANs len = %d, want 3", len(got.L2Bridge.VLANs))
	}
	wantVIDs := []uint16{100, 200, 300}
	for i, want := range wantVIDs {
		if got.L2Bridge.VLANs[i].VID != want {
			t.Errorf("VLANs[%d].VID = %d, want %d", i, got.L2Bridge.VLANs[i].VID, want)
		}
	}
	// Multi-subnet VLAN entry
	multi := got.L2Bridge.VLANs[2].Subnets
	if len(multi) != 2 || multi[0] != "10.30.0.0/16" || multi[1] != "10.31.0.0/16" {
		t.Errorf("VLANs[2].Subnets = %v, want [10.30.0.0/16 10.31.0.0/16]", multi)
	}
}

func TestGetNetwork_InvalidVLANID(t *testing.T) {
	cases := []struct {
		name string
		vlan map[string]interface{} // value placed at spec.l2Bridge.vlans[0].vlan
	}{
		{
			name: "missing id",
			vlan: map[string]interface{}{},
		},
		{
			name: "negative id",
			vlan: map[string]interface{}{"id": int64(-1)},
		},
		{
			name: "id above range",
			vlan: map[string]interface{}{"id": int64(5000)},
		},
		{
			name: "id is reserved zero",
			vlan: map[string]interface{}{"id": int64(0)},
		},
		{
			name: "non-integer float id",
			vlan: map[string]interface{}{"id": 1.5},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			nw := makeNetwork("bad-vlan", map[string]interface{}{
				"l2Bridge": map[string]interface{}{
					"vlans": []interface{}{
						map[string]interface{}{
							"vlan":    tc.vlan,
							"subnets": []interface{}{map[string]interface{}{"cidr": "10.30.0.0/16"}},
						},
					},
				},
			})
			cb, _ := newFakeClientWith(nw)
			c := cb.Build()

			if _, err := GetNetwork(context.Background(), c, "bad-vlan"); err == nil {
				t.Fatalf("expected error, got nil")
			}
		})
	}
}
