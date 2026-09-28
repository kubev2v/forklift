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

func newVsphereColdPredicate(t *testing.T, c client.Client, storageClass string, netAppShift bool) *BasePredicate {
	t.Helper()
	vsphere, openshift := api.VSphere, api.OpenShift
	return &BasePredicate{
		vm: &plan.VM{Ref: ref.Ref{ID: "vm-1"}},
		context: &plancontext.Context{
			Plan: &api.Plan{
				Spec: api.PlanSpec{MigrateSharedDisks: true},
				// Simulates the result validateNetAppShift persists once per reconcile,
				// during plan validation, before ShouldUseV2vForTransfer is ever called.
				Status: api.PlanStatus{NetAppShiftDestination: netAppShift},
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

// ShouldUseV2vForTransfer must never query the API itself: the NetApp Shift destination
// check is resolved once per reconcile by plan validation and persisted on
// Plan.Status.NetAppShiftDestination, so BasePredicate's own cache (and repeat Evaluate
// calls) should never trigger a destination Get.
func TestBasePredicate_useV2vForTransferCached(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = storagev1.AddToScheme(scheme)
	cl := &getCountingClient{Client: fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(&storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: "sc1"}}).
		Build()}

	pred := newVsphereColdPredicate(t, cl, "sc1", false)

	cdiAllowed, err := pred.Evaluate(CDIDiskCopy)
	if err != nil {
		t.Fatal(err)
	}
	v2vAllowed, err := pred.Evaluate(VirtV2vDiskCopy)
	if err != nil {
		t.Fatal(err)
	}
	if cdiAllowed || !v2vAllowed {
		t.Fatalf("netAppShift=false: CDIDiskCopy allowed = %v (want false), VirtV2vDiskCopy allowed = %v (want true)", cdiAllowed, v2vAllowed)
	}
	if cl.n.Load() != 0 {
		t.Fatalf("destination Get calls = %d, want 0 (never queried; must use cached Status.NetAppShiftDestination)", cl.n.Load())
	}
}

// When the destination is a NetApp Shift StorageClass, ShouldUseV2vForTransfer must reject
// virt-v2v transfer (CDI handles the copy instead), and this must still be decided purely
// from the cached Status.NetAppShiftDestination without ever querying the destination.
func TestBasePredicate_useV2vForTransferNetAppShift(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = storagev1.AddToScheme(scheme)
	cl := &getCountingClient{Client: fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(&storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: "sc1"}}).
		Build()}

	pred := newVsphereColdPredicate(t, cl, "sc1", true)

	cdiAllowed, err := pred.Evaluate(CDIDiskCopy)
	if err != nil {
		t.Fatal(err)
	}
	v2vAllowed, err := pred.Evaluate(VirtV2vDiskCopy)
	if err != nil {
		t.Fatal(err)
	}
	if !cdiAllowed || v2vAllowed {
		t.Fatalf("netAppShift=true: CDIDiskCopy allowed = %v (want true), VirtV2vDiskCopy allowed = %v (want false)", cdiAllowed, v2vAllowed)
	}
	if cl.n.Load() != 0 {
		t.Fatalf("destination Get calls = %d, want 0 (never queried; must use cached Status.NetAppShiftDestination)", cl.n.Load())
	}
}

func newBaseMigratorWithProvider(t *testing.T, p *api.Plan, migration *api.Migration) *BaseMigrator {
	t.Helper()
	vsphere, openshift := api.VSphere, api.OpenShift

	scheme := runtime.NewScheme()
	if err := storagev1.AddToScheme(scheme); err != nil {
		t.Fatalf("register storage types: %v", err)
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()

	ctx := &plancontext.Context{
		Plan:      p,
		Migration: migration,
		Source: plancontext.Source{
			Provider: &api.Provider{Spec: api.ProviderSpec{Type: &vsphere, URL: "https://vc"}},
		},
		Destination: plancontext.Destination{
			Client: cl,
		},
	}

	if p.Provider.Source == nil {
		p.Provider.Source = &api.Provider{Spec: api.ProviderSpec{Type: &vsphere, URL: "https://vc"}}
	}
	if p.Provider.Destination == nil {
		p.Provider.Destination = &api.Provider{Spec: api.ProviderSpec{Type: &openshift, URL: ""}}
	}

	return &BaseMigrator{Context: ctx}
}

func TestItinerary_ResumeConversion_SelectsResumeConversion(t *testing.T) {
	p := &api.Plan{Spec: api.PlanSpec{Warm: true}}
	m := &api.Migration{}
	m.Spec.ResumeConversion = true
	migrator := newBaseMigratorWithProvider(t, p, m)

	vm := plan.VM{Ref: ref.Ref{ID: "vm-1"}}
	itr := migrator.Itinerary(vm)

	if itr.Name != "ResumeConversion" {
		t.Fatalf("expected ResumeConversion itinerary, got %q", itr.Name)
	}

	phases := map[string]bool{}
	for _, step := range itr.Pipeline {
		phases[step.Name] = true
	}

	excluded := []string{
		api.PhaseCopyDisks,
		api.PhaseCopyDisksVirtV2V,
		api.PhaseStorePowerState,
		api.PhasePowerOffSource,
	}
	for _, p := range excluded {
		if phases[p] {
			t.Errorf("resume-conversion itinerary should not contain %q", p)
		}
	}

	required := []string{
		api.PhaseCreateGuestConversionPod,
		api.PhaseConvertGuest,
		api.PhaseCreateVM,
		api.PhaseCompleted,
	}
	for _, p := range required {
		if !phases[p] {
			t.Errorf("resume-conversion itinerary missing required phase %q", p)
		}
	}
}

func TestItinerary_ConversionOnlyPlanType(t *testing.T) {
	p := &api.Plan{Spec: api.PlanSpec{Type: api.MigrationOnlyConversion}}
	migrator := newBaseMigratorWithProvider(t, p, nil)

	vm := plan.VM{Ref: ref.Ref{ID: "vm-1"}}
	itr := migrator.Itinerary(vm)

	if itr.Name != "OnlyConversion" {
		t.Fatalf("expected OnlyConversion itinerary for conversion-only plan type, got %q", itr.Name)
	}
}

func TestItinerary_NormalWarm_SelectsWarm(t *testing.T) {
	p := &api.Plan{Spec: api.PlanSpec{Warm: true}}
	migrator := newBaseMigratorWithProvider(t, p, nil)

	vm := plan.VM{Ref: ref.Ref{ID: "vm-1"}}
	itr := migrator.Itinerary(vm)

	if itr.Name != "Warm" {
		t.Fatalf("expected Warm itinerary, got %q", itr.Name)
	}
}

func TestItinerary_NilMigration_DoesNotSelectConversion(t *testing.T) {
	p := &api.Plan{}
	migrator := newBaseMigratorWithProvider(t, p, nil)

	vm := plan.VM{Ref: ref.Ref{ID: "vm-1"}}
	itr := migrator.Itinerary(vm)

	if itr.Name == "OnlyConversion" || itr.Name == "Warm" {
		t.Fatalf("expected default (cold) itinerary with nil migration, got %q", itr.Name)
	}
}

func TestReset_ResumeConversion_PreservesState(t *testing.T) {
	p := &api.Plan{Spec: api.PlanSpec{Warm: true}}
	m := &api.Migration{}
	m.Spec.ResumeConversion = true
	migrator := newBaseMigratorWithProvider(t, p, m)

	vmStatus := &plan.VMStatus{
		VM:          plan.VM{Ref: ref.Ref{ID: "vm-1"}},
		DisksCopied: true,
	}
	pipeline := []*plan.Step{}

	migrator.Reset(vmStatus, pipeline)

	if vmStatus.Warm != nil {
		t.Fatal("expected Warm to remain nil during resume-conversion reset")
	}
	if !vmStatus.DisksCopied {
		t.Fatal("expected DisksCopied to be preserved during reset")
	}
	if vmStatus.Phase != api.PhaseStarted {
		t.Fatalf("expected phase Started, got %q", vmStatus.Phase)
	}
}

func TestReset_NormalWarm_SetsWarm(t *testing.T) {
	p := &api.Plan{Spec: api.PlanSpec{Warm: true}}
	migrator := newBaseMigratorWithProvider(t, p, nil)

	vmStatus := &plan.VMStatus{
		VM: plan.VM{Ref: ref.Ref{ID: "vm-1"}},
	}
	pipeline := []*plan.Step{}

	migrator.Reset(vmStatus, pipeline)

	if vmStatus.Warm == nil {
		t.Fatal("expected Warm to be set during normal warm reset")
	}
}
