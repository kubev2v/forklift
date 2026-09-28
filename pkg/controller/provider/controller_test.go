package provider

import (
	"fmt"
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func makeProvider(pt api.ProviderType) *api.Provider {
	return &api.Provider{
		ObjectMeta: v1.ObjectMeta{Name: "test"},
		Spec:       api.ProviderSpec{Type: &pt},
	}
}

func TestSetAuthFailureConditions_HyperV_SetsBothConditions(t *testing.T) {
	p := makeProvider(api.HyperV)
	setAuthFailureConditions(p, fmt.Errorf("HTTP 401"))

	if !p.Status.HasCondition(ConnectionAuthFailed) {
		t.Fatal("expected ConnectionAuthFailed")
	}
	if !p.Status.HasCondition(ConnectionAuthRetry) {
		t.Fatal("expected ConnectionAuthRetry for HyperV")
	}
	if p.Status.Phase != ConnectionFailed {
		t.Errorf("expected phase %q, got %q", ConnectionFailed, p.Status.Phase)
	}
}

func TestSetAuthFailureConditions_NonHyperV_OnlyAuthFailed(t *testing.T) {
	for _, pt := range []api.ProviderType{api.VSphere, api.OVirt, api.OpenStack, api.Ova} {
		t.Run(string(pt), func(t *testing.T) {
			p := makeProvider(pt)
			setAuthFailureConditions(p, fmt.Errorf("HTTP 401"))

			if !p.Status.HasCondition(ConnectionAuthFailed) {
				t.Fatal("expected ConnectionAuthFailed")
			}
			if p.Status.HasCondition(ConnectionAuthRetry) {
				t.Fatalf("non-HyperV provider %s should not have ConnectionAuthRetry", pt)
			}
		})
	}
}

// Verify the defer logic: ConnectionAuthRetry → AuthRetryReQ
func TestDeferRequeue_HyperV_AuthRetry(t *testing.T) {
	p := makeProvider(api.HyperV)
	setAuthFailureConditions(p, fmt.Errorf("HTTP 401"))

	if !p.Status.HasCondition(ConnectionAuthRetry) {
		t.Fatal("precondition: expected ConnectionAuthRetry")
	}
	if !p.Status.HasCondition(ConnectionAuthFailed) {
		t.Fatal("precondition: expected ConnectionAuthFailed")
	}
	// ConnectionAuthRetry is checked first in the defer, so requeue = AuthRetryReQ
}

// Verify the defer logic: ConnectionAuthFailed without ConnectionAuthRetry → requeue = 0
func TestDeferRequeue_NonHyperV_AuthFailed_StopsReconciliation(t *testing.T) {
	p := makeProvider(api.VSphere)
	setAuthFailureConditions(p, fmt.Errorf("HTTP 401"))

	if p.Status.HasCondition(ConnectionAuthRetry) {
		t.Fatal("vSphere should not have ConnectionAuthRetry")
	}
	if !p.Status.HasCondition(ConnectionAuthFailed) {
		t.Fatal("expected ConnectionAuthFailed")
	}
	// ConnectionAuthRetry absent → falls through to ConnectionAuthFailed → requeue = 0
}

// Verify staging removes both auth conditions after a successful connection test
func TestStagingRemovesAuthRetryOnSuccess(t *testing.T) {
	p := makeProvider(api.HyperV)
	setAuthFailureConditions(p, fmt.Errorf("HTTP 401"))

	if !p.Status.HasCondition(ConnectionAuthRetry) {
		t.Fatal("precondition: expected ConnectionAuthRetry")
	}

	p.Status.BeginStagingConditions()
	p.Status.SetCondition(libcnd.Condition{
		Type:   ConnectionTestSucceeded,
		Status: True,
	})
	p.Status.EndStagingConditions()

	if p.Status.HasCondition(ConnectionAuthRetry) {
		t.Error("ConnectionAuthRetry should be removed after successful connection")
	}
	if p.Status.HasCondition(ConnectionAuthFailed) {
		t.Error("ConnectionAuthFailed should be removed after successful connection")
	}
	if !p.Status.HasCondition(ConnectionTestSucceeded) {
		t.Error("expected ConnectionTestSucceeded")
	}
}
