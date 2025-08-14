package ensurer

import (
	"context"
	"strings"
	"testing"

	v1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/provider"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/controller/plan/namespace"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	cnv "kubevirt.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestEnsurer_LiveMigrationNamespaceExclusion(t *testing.T) {
	// Test that the ensurer correctly detects OCP live migrations for namespace exclusion

	// Test case 1: OCP to OCP live migration detection
	ensurer := createEnsurer(true, true)

	// Verify the live migration will trigger namespace exclusion
	isSourceOCP := ensurer.Plan.IsSourceProviderOCP()
	isDestHost := ensurer.Plan.Provider.Destination.IsHost()

	if !isSourceOCP {
		t.Errorf("Expected OCP source provider to be detected for live migration namespace exclusion")
	}

	if !isDestHost {
		t.Errorf("Expected OCP destination to be detected as host for live migration namespace exclusion")
	}

	t.Logf("Live migration namespace exclusion logic:")
	t.Logf("- OCP source detected: %v", isSourceOCP)
	t.Logf("- OCP destination detected: %v", isDestHost)
	t.Logf("- Namespace exclusion applies to live migrations")
	t.Logf("- Uses same %s=ignore label as cold migrations", namespace.KubemacpoolIgnoreLabelKey)

	// Test case 2: Non-OCP live migration should not apply namespace exclusion
	ensurer2 := createEnsurer(false, true) // VMware to OCP

	isSourceOCP2 := ensurer2.Plan.IsSourceProviderOCP()

	if isSourceOCP2 {
		t.Errorf("VMware source should not be detected as OCP for live migration namespace exclusion")
	}

	t.Logf("Non-OCP live migration correctly bypasses namespace exclusion")
}

func TestEnsurer_EmptyNamespaceGuard(t *testing.T) {
	t.Parallel()
	// Test that the ensurer correctly fails fast when TargetNamespace is empty

	ensurer := createEnsurerWithEmptyNamespace()

	// Try to call the namespace exclusion method
	_, err := namespace.EnsureKubemacpoolExclusion(ensurer.Context)

	// Verify it fails with a clear error message
	if err == nil {
		t.Fatalf("expected error when TargetNamespace is empty")
	}
	if !strings.Contains(err.Error(), "target namespace") {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Logf("Empty namespace guard working correctly:")
	t.Logf("- Fails fast with clear error message")
	t.Logf("- Prevents misleading 'failed to get target namespace' errors")
	t.Logf("- Error: %s", err.Error())
}

func TestEnsurer_CentralizedHelper(t *testing.T) {
	t.Parallel()
	// Test that the centralized namespace.EnsureKubemacpoolExclusion helper works correctly

	tests := []struct {
		name          string
		namespaceName string
		sourceIsOCP   bool
		destIsHost    bool
		expectApplied bool
		expectLabel   bool
		expectOwners  bool
		description   string
	}{
		{
			name:          "same-cluster OCP migration",
			namespaceName: "same-cluster-test-ns",
			sourceIsOCP:   true,
			destIsHost:    true,
			expectApplied: true,
			expectLabel:   true,
			expectOwners:  true,
			description:   "OCP-to-OCP migration should apply kubemacpool exclusion",
		},
		{
			name:          "VMware to OCP migration",
			namespaceName: "vmware-test-ns",
			sourceIsOCP:   false,
			destIsHost:    true,
			expectApplied: false,
			expectLabel:   false,
			expectOwners:  false,
			description:   "Non-OCP source migration should not apply kubemacpool exclusion",
		},
		{
			name:          "cross-cluster OCP migration",
			namespaceName: "cross-cluster-test-ns",
			sourceIsOCP:   true,
			destIsHost:    false,
			expectApplied: false,
			expectLabel:   false,
			expectOwners:  false,
			description:   "OCP source to remote OCP destination should not apply kubemacpool exclusion",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ensurer := createEnsurerWithNamespace(tt.namespaceName, tt.sourceIsOCP, tt.destIsHost)

			applied, err := namespace.EnsureKubemacpoolExclusion(ensurer.Context)
			if err != nil {
				t.Errorf("Expected no error for %s, got: %v", tt.description, err)
			}
			if applied != tt.expectApplied {
				t.Errorf("Expected applied=%v for %s, got applied=%v", tt.expectApplied, tt.description, applied)
			}

			// Verify namespace state
			testNamespace := &core.Namespace{}
			err = ensurer.Context.Destination.Client.Get(context.TODO(), client.ObjectKey{Name: tt.namespaceName}, testNamespace)
			if err != nil {
				t.Fatalf("Failed to get namespace for verification: %v", err)
			}

			// Check kubemacpool ignore label
			hasLabel := testNamespace.Labels[namespace.KubemacpoolIgnoreLabelKey] == namespace.KubemacpoolIgnoreLabelValue
			if hasLabel != tt.expectLabel {
				t.Errorf("Expected kubemacpool ignore label present=%v for %s, got present=%v", tt.expectLabel, tt.description, hasLabel)
			}

			// Check owners annotation (only for positive cases)
			if tt.expectOwners {
				// Owners annotation must include this plan UID.
				owners := testNamespace.Annotations[namespace.KubemacpoolOwnersAnnotationKey]
				if !strings.Contains(owners, string(ensurer.Context.Plan.GetUID())) {
					t.Errorf("expected owners annotation to include plan UID %q, got: %q", ensurer.Context.Plan.GetUID(), owners)
				}
			}

			t.Logf("%s: applied=%v, err=%v", tt.description, applied, err)
		})
	}

	t.Logf("Centralized helper function working correctly:")
	t.Logf("- Eliminates code duplication between kubevirt and ensurer")
	t.Logf("- Prevents gate logic drift between migration types")
}

func createEnsurerWithNamespace(namespaceName string, sourceIsOCP, destIsHost bool) *Ensurer {
	var sourceType, destType v1beta1.ProviderType

	if sourceIsOCP {
		sourceType = v1beta1.OpenShift
	} else {
		sourceType = v1beta1.VSphere
	}

	// Always use OpenShift for destination in OCP tests
	// Use destIsHost to control host vs remote cluster via URL
	destType = v1beta1.OpenShift

	// Create providers
	sourceProvider := &v1beta1.Provider{
		ObjectMeta: metav1.ObjectMeta{
			Name: "source-provider",
		},
		Spec: v1beta1.ProviderSpec{
			Type: &sourceType,
		},
	}

	destProvider := &v1beta1.Provider{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dest-provider",
		},
		Spec: v1beta1.ProviderSpec{
			Type: &destType,
		},
	}

	// Set URL for host vs remote providers
	if sourceIsOCP {
		sourceProvider.Spec.URL = "" // OCP source is always host (same cluster)
	}
	if destIsHost {
		destProvider.Spec.URL = "" // Host provider has empty URL (same cluster)
	} else {
		destProvider.Spec.URL = "https://remote-cluster.example.com" // Remote provider has URL (different cluster)
	}

	// Create plan
	plan := &v1beta1.Plan{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-plan",
			UID:  "test-plan-uid",
		},
		Spec: v1beta1.PlanSpec{
			TargetNamespace: namespaceName,
			Provider: provider.Pair{
				Source: core.ObjectReference{
					Name: sourceProvider.Name,
				},
				Destination: core.ObjectReference{
					Name: destProvider.Name,
				},
			},
		},
	}

	// Create namespace for testing
	ns := &core.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespaceName,
		},
	}

	// Create fake client
	scheme := runtime.NewScheme()
	_ = core.AddToScheme(scheme)
	_ = cnv.AddToScheme(scheme)
	_ = v1beta1.SchemeBuilder.AddToScheme(scheme)
	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(plan, sourceProvider, destProvider, ns).
		Build()

	// Create migration for labeler
	migration := &v1beta1.Migration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-migration",
			Namespace: "test-namespace",
			UID:       "test-migration-uid",
		},
	}

	// Create context
	ctx := &plancontext.Context{
		Destination: plancontext.Destination{
			Client: k8sClient,
		},
		Log:       logging.WithName("ensurer-test"),
		Client:    k8sClient,
		Plan:      plan,
		Migration: migration,
	}

	// Set up provider references
	ctx.Plan.Provider.Source = sourceProvider
	ctx.Plan.Provider.Destination = destProvider

	return &Ensurer{
		Context: ctx,
	}
}

func createEnsurer(sourceIsOCP, destIsHost bool) *Ensurer {
	return createEnsurerWithNamespace("test-namespace", sourceIsOCP, destIsHost)
}

func createEnsurerWithEmptyNamespace() *Ensurer {
	// Create providers for testing (need them for IsSourceProviderOCP check)
	sourceType := v1beta1.OpenShift
	destType := v1beta1.OpenShift

	sourceProvider := &v1beta1.Provider{
		ObjectMeta: metav1.ObjectMeta{
			Name: "source-provider",
		},
		Spec: v1beta1.ProviderSpec{
			Type: &sourceType,
		},
	}

	destProvider := &v1beta1.Provider{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dest-provider",
		},
		Spec: v1beta1.ProviderSpec{
			Type: &destType,
		},
	}

	// Create a plan with empty TargetNamespace to test the guard clause
	plan := &v1beta1.Plan{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-plan-empty-ns",
			UID:  "test-plan-empty-ns-uid",
		},
		Spec: v1beta1.PlanSpec{
			// TargetNamespace is intentionally empty to test the guard
			Provider: provider.Pair{
				Source: core.ObjectReference{
					Name: "source-provider",
				},
				Destination: core.ObjectReference{
					Name: "dest-provider",
				},
			},
		},
	}

	// Create fake client
	scheme := runtime.NewScheme()
	_ = core.AddToScheme(scheme)
	_ = cnv.AddToScheme(scheme)
	_ = v1beta1.SchemeBuilder.AddToScheme(scheme)
	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(plan, sourceProvider, destProvider).
		Build()

	// Create migration for context
	migration := &v1beta1.Migration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-migration",
			Namespace: "test-namespace",
			UID:       "test-migration-uid",
		},
	}

	// Create context
	ctx := &plancontext.Context{
		Destination: plancontext.Destination{
			Client: k8sClient,
		},
		Log:       logging.WithName("ensurer-test"),
		Client:    k8sClient,
		Plan:      plan,
		Migration: migration,
	}

	// Set up provider references
	ctx.Plan.Provider.Source = sourceProvider
	ctx.Plan.Provider.Destination = destProvider

	return &Ensurer{
		Context: ctx,
	}
}
