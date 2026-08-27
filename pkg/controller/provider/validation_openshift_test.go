package provider

import (
	"fmt"
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/base"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stesting "k8s.io/client-go/testing"

	"k8s.io/client-go/discovery"
	discoveryfake "k8s.io/client-go/discovery/fake"
	clientsetfake "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const openshiftTestNamespace = "test"

func openshiftTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := api.SchemeBuilder.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := core.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	return scheme
}

func newOpenshiftReconciler(t *testing.T) Reconciler {
	t.Helper()
	scheme := openshiftTestScheme(t)
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	return Reconciler{Reconciler: base.Reconciler{Client: cl}}
}

// remoteOpenShiftProvider returns a non-host OpenShift provider that has already
// passed the connection test (so ValidateForkliftInstalled will run).
func remoteOpenShiftProvider() *api.Provider {
	pt := api.OpenShift
	p := &api.Provider{
		ObjectMeta: metav1.ObjectMeta{Name: "remote", Namespace: openshiftTestNamespace},
		Spec: api.ProviderSpec{
			Type: &pt,
			URL:  "https://remote.example.com:6443",
		},
	}
	p.Status.SetCondition(libcnd.Condition{
		Type:     ConnectionTestSucceeded,
		Status:   True,
		Category: Required,
	})
	return p
}

// fakeDiscovery installs a stubbed discovery client for the duration of a test.
func fakeDiscovery(t *testing.T, resources []*metav1.APIResourceList, reactErr error) {
	t.Helper()
	orig := discoveryClientForProvider
	t.Cleanup(func() { discoveryClientForProvider = orig })

	discoveryClientForProvider = func(_ *api.Provider, _ *core.Secret) (discovery.DiscoveryInterface, error) {
		cs := clientsetfake.NewSimpleClientset()
		fd := cs.Discovery().(*discoveryfake.FakeDiscovery)
		fd.Resources = resources
		if reactErr != nil {
			fd.PrependReactor("get", "resource", func(k8stesting.Action) (bool, runtime.Object, error) {
				return true, nil, reactErr
			})
		}
		return fd, nil
	}
}

func forkliftGroupWithController() []*metav1.APIResourceList {
	return []*metav1.APIResourceList{
		{
			GroupVersion: api.SchemeGroupVersion.String(),
			APIResources: []metav1.APIResource{
				{Name: "providers", Kind: "Provider"},
				{Name: forkliftControllerResource, Kind: "ForkliftController"},
			},
		},
	}
}

func TestValidateForkliftInstalled_OperatorPresent(t *testing.T) {
	fakeDiscovery(t, forkliftGroupWithController(), nil)
	r := newOpenshiftReconciler(t)
	p := remoteOpenShiftProvider()

	if err := r.ValidateForkliftInstalled(p, &core.Secret{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Status.HasCondition(ForkliftNotInstalled) {
		t.Fatal("did not expect ForkliftNotInstalled when the operator CRD is present")
	}
}

func TestValidateForkliftInstalled_GroupMissing(t *testing.T) {
	// No forklift group served at all.
	fakeDiscovery(t, nil, nil)
	r := newOpenshiftReconciler(t)
	p := remoteOpenShiftProvider()

	if err := r.ValidateForkliftInstalled(p, &core.Secret{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cnd := p.Status.FindCondition(ForkliftNotInstalled)
	if cnd == nil {
		t.Fatal("expected ForkliftNotInstalled condition when the API group is absent")
	}
	if cnd.Category != Warn {
		t.Errorf("expected Warn category, got %q", cnd.Category)
	}
}

func TestValidateForkliftInstalled_GroupPresentButNoController(t *testing.T) {
	resources := []*metav1.APIResourceList{
		{
			GroupVersion: api.SchemeGroupVersion.String(),
			APIResources: []metav1.APIResource{{Name: "providers", Kind: "Provider"}},
		},
	}
	fakeDiscovery(t, resources, nil)
	r := newOpenshiftReconciler(t)
	p := remoteOpenShiftProvider()

	if err := r.ValidateForkliftInstalled(p, &core.Secret{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !p.Status.HasCondition(ForkliftNotInstalled) {
		t.Fatal("expected ForkliftNotInstalled when the operator CRD is absent from the group")
	}
}

func TestValidateForkliftInstalled_StaleConditionCleared(t *testing.T) {
	fakeDiscovery(t, forkliftGroupWithController(), nil)
	r := newOpenshiftReconciler(t)
	p := remoteOpenShiftProvider()
	// Pre-existing stale warning from a prior reconcile.
	p.Status.SetCondition(libcnd.Condition{Type: ForkliftNotInstalled, Status: True, Category: Warn})

	if err := r.ValidateForkliftInstalled(p, &core.Secret{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Status.HasCondition(ForkliftNotInstalled) {
		t.Fatal("expected stale ForkliftNotInstalled condition to be cleared")
	}
}

func TestValidateForkliftInstalled_TransientErrorRequeues(t *testing.T) {
	fakeDiscovery(t, nil, fmt.Errorf("connection refused"))
	r := newOpenshiftReconciler(t)
	p := remoteOpenShiftProvider()

	err := r.ValidateForkliftInstalled(p, &core.Secret{})
	if err == nil {
		t.Fatal("expected a non-nil error to trigger a requeue on transient failure")
	}
	if p.Status.HasCondition(ForkliftNotInstalled) {
		t.Fatal("did not expect ForkliftNotInstalled to be set on a transient error")
	}
}

func TestValidateForkliftInstalled_HostProviderSkipped(t *testing.T) {
	fakeDiscovery(t, nil, nil) // would report not-installed if it ran
	r := newOpenshiftReconciler(t)
	pt := api.OpenShift
	p := &api.Provider{
		ObjectMeta: metav1.ObjectMeta{Name: "host", Namespace: openshiftTestNamespace},
		Spec:       api.ProviderSpec{Type: &pt}, // no URL => host provider
	}

	if err := r.ValidateForkliftInstalled(p, &core.Secret{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Status.HasCondition(ForkliftNotInstalled) {
		t.Fatal("host provider should be skipped, no condition expected")
	}
}

func TestValidateForkliftInstalled_SkippedWithoutConnection(t *testing.T) {
	fakeDiscovery(t, nil, nil)
	r := newOpenshiftReconciler(t)
	pt := api.OpenShift
	p := &api.Provider{
		ObjectMeta: metav1.ObjectMeta{Name: "remote", Namespace: openshiftTestNamespace},
		Spec:       api.ProviderSpec{Type: &pt, URL: "https://remote.example.com:6443"},
	}
	// No ConnectionTestSucceeded condition set.

	if err := r.ValidateForkliftInstalled(p, &core.Secret{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Status.HasCondition(ForkliftNotInstalled) {
		t.Fatal("should skip the check until the connection test has succeeded")
	}
}
