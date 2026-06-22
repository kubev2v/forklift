package base

import (
	"context"
	"sync/atomic"
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type getCountingClient struct {
	client.Client
	n atomic.Int32
}

func (c *getCountingClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	c.n.Add(1)
	return c.Client.Get(ctx, key, obj, opts...)
}

func newVsphereColdPredicate(t *testing.T, c client.Client, storageClass string) *BasePredicate {
	t.Helper()
	vsphere, openshift := api.VSphere, api.OpenShift
	return &BasePredicate{
		vm: &plan.VM{Ref: ref.Ref{ID: "vm-1"}},
		context: &plancontext.Context{
			Plan: &api.Plan{
				Spec: api.PlanSpec{MigrateSharedDisks: true},
				Referenced: api.Referenced{
					Provider: struct {
						Source, Destination *api.Provider
					}{
						Source:      &api.Provider{Spec: api.ProviderSpec{Type: &vsphere, URL: "https://vc"}},
						Destination: &api.Provider{Spec: api.ProviderSpec{Type: &openshift, URL: ""}},
					},
					Map: struct {
						Network *api.NetworkMap
						Storage *api.StorageMap
					}{
						Storage: &api.StorageMap{
							Spec: api.StorageMapSpec{
								Map: []api.StoragePair{{Destination: api.DestinationStorage{StorageClass: storageClass}}},
							},
						},
					},
				},
			},
			Destination: plancontext.Destination{Client: c},
		},
	}
}

// ShouldUseV2vForTransfer hits the API once per predicate; second Evaluate must reuse the cache.
func TestBasePredicate_useV2vForTransferCached(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = storagev1.AddToScheme(scheme)
	cl := &getCountingClient{Client: fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(&storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: "sc1"}}).
		Build()}

	pred := newVsphereColdPredicate(t, cl, "sc1")

	if _, err := pred.Evaluate(CDIDiskCopy); err != nil {
		t.Fatal(err)
	}
	if _, err := pred.Evaluate(VirtV2vDiskCopy); err != nil {
		t.Fatal(err)
	}
	if cl.n.Load() != 1 {
		t.Fatalf("destination Get calls = %d, want 1 (cached)", cl.n.Load())
	}
}
