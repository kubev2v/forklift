package namespace

import (
	"context"
	"fmt"
	"strings"
	"testing"

	v1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestEnsureKubemacpoolExclusion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		sourceIsOCP     bool
		sourceIsHost    bool
		destIsOCP       bool
		destIsHost      bool
		namespaceExists bool
		existingLabel   string
		expectApplied   bool
		expectError     bool
	}{
		{
			name:            "Same-cluster OCP migration applies exclusion",
			sourceIsOCP:     true,
			sourceIsHost:    true,
			destIsOCP:       true,
			destIsHost:      true,
			namespaceExists: true,
			expectApplied:   true,
			expectError:     false,
		},
		{
			name:            "Same-cluster OCP migration with existing ignore label",
			sourceIsOCP:     true,
			sourceIsHost:    true,
			destIsOCP:       true,
			destIsHost:      true,
			namespaceExists: true,
			existingLabel:   KubemacpoolIgnoreLabelValue,
			expectApplied:   true,
			expectError:     false,
		},
		{
			name:            "Existing non-ignore label is normalized to ignore",
			sourceIsOCP:     true,
			sourceIsHost:    true,
			destIsOCP:       true,
			destIsHost:      true,
			namespaceExists: true,
			existingLabel:   "some-other-value",
			expectApplied:   true,
			expectError:     false,
		},
		{
			name:          "Cross-cluster OCP migration skips exclusion",
			sourceIsOCP:   true,
			sourceIsHost:  true,
			destIsOCP:     true,
			destIsHost:    false, // Remote destination
			expectApplied: false,
			expectError:   false,
		},
		{
			name:          "VMware source migration skips exclusion",
			sourceIsOCP:   false,
			sourceIsHost:  false,
			destIsOCP:     true,
			destIsHost:    true,
			expectApplied: false,
			expectError:   false,
		},
		{
			name:            "Missing namespace returns error",
			sourceIsOCP:     true,
			sourceIsHost:    true,
			destIsOCP:       true,
			destIsHost:      true,
			namespaceExists: false,
			expectApplied:   false,
			expectError:     true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create test context
			ctx := createTestContext(tt.sourceIsOCP, tt.sourceIsHost, tt.destIsOCP, tt.destIsHost, tt.namespaceExists, tt.existingLabel)

			// Set a UID for the plan for reference counting
			ctx.Plan.SetUID(types.UID("ensure-test-" + strings.ReplaceAll(tt.name, " ", "-")))

			// Execute
			applied, err := EnsureKubemacpoolExclusion(ctx)

			// Verify function result
			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if applied != tt.expectApplied {
				t.Errorf("Expected applied=%v, got %v", tt.expectApplied, applied)
			}

			// Verify actual namespace state (when namespace exists and no error expected)
			if tt.namespaceExists && !tt.expectError {
				namespace := &core.Namespace{}
				err := ctx.Destination.Client.Get(context.TODO(), k8sclient.ObjectKey{Name: "test-namespace"}, namespace)
				if err != nil {
					t.Fatalf("Failed to get namespace for state verification: %v", err)
				}

				if tt.expectApplied {
					// Should have the kubemacpool exclusion label
					if namespace.Labels == nil {
						t.Errorf("Expected namespace to have labels after applying exclusion")
					} else if value, exists := namespace.Labels[KubemacpoolIgnoreLabelKey]; !exists {
						t.Errorf("Expected namespace to have kubemacpool exclusion label key")
					} else if value != KubemacpoolIgnoreLabelValue {
						t.Errorf("Expected kubemacpool label value '%s', got '%s'", KubemacpoolIgnoreLabelValue, value)
					}
				}
			}
		})
	}
}

func TestRemoveKubemacpoolExclusion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		sourceIsOCP     bool
		sourceIsHost    bool
		destIsOCP       bool
		destIsHost      bool
		namespaceExists bool
		existingLabel   string
		expectRemoved   bool
		expectError     bool
	}{
		{
			name:            "Same-cluster OCP migration removes label applied by this plan",
			sourceIsOCP:     true,
			sourceIsHost:    true,
			destIsOCP:       true,
			destIsHost:      true,
			namespaceExists: true,
			existingLabel:   "", // Start without label
			expectRemoved:   true,
			expectError:     false,
		},
		{
			name:            "Same-cluster OCP migration with no label to remove",
			sourceIsOCP:     true,
			sourceIsHost:    true,
			destIsOCP:       true,
			destIsHost:      true,
			namespaceExists: true,
			existingLabel:   "", // No label
			expectRemoved:   false,
			expectError:     false,
		},
		{
			name:          "Cross-cluster OCP migration skips removal",
			sourceIsOCP:   true,
			sourceIsHost:  true,
			destIsOCP:     true,
			destIsHost:    false, // Remote destination
			expectRemoved: false,
			expectError:   false,
		},
		{
			name:          "VMware source migration skips removal",
			sourceIsOCP:   false,
			sourceIsHost:  false,
			destIsOCP:     true,
			destIsHost:    true,
			expectRemoved: false,
			expectError:   false,
		},
		{
			name:            "Missing namespace returns error",
			sourceIsOCP:     true,
			sourceIsHost:    true,
			destIsOCP:       true,
			destIsHost:      true,
			namespaceExists: false,
			expectRemoved:   false,
			expectError:     true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create test context
			ctx := createTestContext(tt.sourceIsOCP, tt.sourceIsHost, tt.destIsOCP, tt.destIsHost, tt.namespaceExists, tt.existingLabel)

			// Set a UID for the plan for reference counting
			ctx.Plan.SetUID(types.UID("remove-test-" + strings.ReplaceAll(tt.name, " ", "-")))

			// For the test that expects removal, first ensure exclusion so the plan is in the owners list
			if tt.name == "Same-cluster OCP migration removes label applied by this plan" {
				applied, err := EnsureKubemacpoolExclusion(ctx)
				if err != nil {
					t.Fatalf("Failed to ensure exclusion before testing removal: %v", err)
				}
				if !applied {
					t.Fatalf("Expected EnsureKubemacpoolExclusion to apply exclusion")
				}
			}

			// Execute
			removed, err := RemoveKubemacpoolExclusion(ctx)

			// Verify function result
			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if removed != tt.expectRemoved {
				t.Errorf("Expected removed=%v, got %v", tt.expectRemoved, removed)
			}

			// Verify actual namespace state (when namespace exists and no error expected)
			if tt.namespaceExists && !tt.expectError {
				namespace := &core.Namespace{}
				err := ctx.Destination.Client.Get(context.TODO(), k8sclient.ObjectKey{Name: "test-namespace"}, namespace)
				if err != nil {
					t.Fatalf("Failed to get namespace for state verification: %v", err)
				}

				if tt.expectRemoved {
					// Should NOT have the kubemacpool exclusion label after removal
					if namespace.Labels != nil {
						if _, exists := namespace.Labels[KubemacpoolIgnoreLabelKey]; exists {
							t.Errorf("Expected kubemacpool exclusion label to be removed, but it still exists")
						}
					}
				} else if tt.existingLabel == KubemacpoolIgnoreLabelValue {
					// If we started with the label and didn't remove it, it should still be there
					if namespace.Labels == nil {
						t.Errorf("Expected namespace to still have labels when no removal occurred")
					} else if value, exists := namespace.Labels[KubemacpoolIgnoreLabelKey]; !exists {
						t.Errorf("Expected kubemacpool exclusion label to remain when no removal occurred")
					} else if value != KubemacpoolIgnoreLabelValue {
						t.Errorf("Expected kubemacpool label value '%s', got '%s'", KubemacpoolIgnoreLabelValue, value)
					}
				} else {
					// Started without the label and did not remove: it should remain absent
					if namespace.Labels != nil {
						if _, exists := namespace.Labels[KubemacpoolIgnoreLabelKey]; exists {
							t.Errorf("Did not expect kubemacpool exclusion label to be present")
						}
					}
				}
			}
		})
	}
}

func TestEnsureKubemacpoolExclusion_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                 string
		setupContext         func() *plancontext.Context
		expectApplied        bool
		expectError          bool
		expectedErrSubstring string
	}{
		{
			name: "nil source provider returns error",
			setupContext: func() *plancontext.Context {
				ctx := createTestContext(true, true, true, true, true, "")
				ctx.Plan.SetUID(types.UID("ensure-edge-nil-source"))
				ctx.Plan.Referenced.Provider.Source = nil // Make source provider nil
				return ctx
			},
			expectApplied:        false,
			expectError:          true,
			expectedErrSubstring: "provider is not available",
		},
		{
			name: "nil destination provider returns error",
			setupContext: func() *plancontext.Context {
				ctx := createTestContext(true, true, true, true, true, "")
				ctx.Plan.SetUID(types.UID("ensure-edge-nil-dest"))
				ctx.Plan.Referenced.Provider.Destination = nil // Make destination provider nil
				return ctx
			},
			expectApplied:        false,
			expectError:          true,
			expectedErrSubstring: "provider is not available",
		},
		{
			name: "empty target namespace returns error",
			setupContext: func() *plancontext.Context {
				ctx := createTestContext(true, true, true, true, true, "")
				ctx.Plan.SetUID(types.UID("ensure-edge-empty-ns"))
				ctx.Plan.Spec.TargetNamespace = "" // Make target namespace empty
				return ctx
			},
			expectApplied:        false,
			expectError:          true,
			expectedErrSubstring: "target namespace is empty",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Setup
			ctx := tt.setupContext()

			// Execute
			applied, err := EnsureKubemacpoolExclusion(ctx)

			// Verify error expectation
			if tt.expectError && err == nil {
				t.Fatalf("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if tt.expectError && err != nil && tt.expectedErrSubstring != "" {
				if !strings.Contains(err.Error(), tt.expectedErrSubstring) {
					t.Errorf("Expected error to contain '%s', got '%s'", tt.expectedErrSubstring, err.Error())
				}
			}

			// Verify applied expectation
			if applied != tt.expectApplied {
				t.Errorf("Expected applied=%v, got %v", tt.expectApplied, applied)
			}
		})
	}
}

func TestRemoveKubemacpoolExclusion_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                 string
		setupContext         func() *plancontext.Context
		expectRemoved        bool
		expectError          bool
		expectedErrSubstring string
	}{
		{
			name: "nil source provider returns error",
			setupContext: func() *plancontext.Context {
				ctx := createTestContext(true, true, true, true, true, KubemacpoolIgnoreLabelValue)
				ctx.Plan.SetUID(types.UID("remove-edge-nil-source"))
				ctx.Plan.Referenced.Provider.Source = nil // Make source provider nil
				return ctx
			},
			expectRemoved:        false,
			expectError:          true,
			expectedErrSubstring: "provider is not available",
		},
		{
			name: "nil destination provider returns error",
			setupContext: func() *plancontext.Context {
				ctx := createTestContext(true, true, true, true, true, KubemacpoolIgnoreLabelValue)
				ctx.Plan.SetUID(types.UID("remove-edge-nil-dest"))
				ctx.Plan.Referenced.Provider.Destination = nil // Make destination provider nil
				return ctx
			},
			expectRemoved:        false,
			expectError:          true,
			expectedErrSubstring: "provider is not available",
		},
		{
			name: "empty target namespace returns error",
			setupContext: func() *plancontext.Context {
				ctx := createTestContext(true, true, true, true, true, KubemacpoolIgnoreLabelValue)
				ctx.Plan.SetUID(types.UID("remove-edge-empty-ns"))
				ctx.Plan.Spec.TargetNamespace = "" // Make target namespace empty
				return ctx
			},
			expectRemoved:        false,
			expectError:          true,
			expectedErrSubstring: "target namespace is empty",
		},
		{
			name: "nil destination client returns error",
			setupContext: func() *plancontext.Context {
				ctx := createTestContext(true, true, true, true, true, KubemacpoolIgnoreLabelValue)
				ctx.Plan.SetUID(types.UID("remove-edge-nil-client"))
				ctx.Destination.Client = nil // Make destination client nil
				return ctx
			},
			expectRemoved:        false,
			expectError:          true,
			expectedErrSubstring: "destination client is not configured",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Setup
			ctx := tt.setupContext()

			// Execute
			removed, err := RemoveKubemacpoolExclusion(ctx)

			// Verify error expectation
			if tt.expectError && err == nil {
				t.Fatalf("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if tt.expectError && err != nil && tt.expectedErrSubstring != "" {
				if !strings.Contains(err.Error(), tt.expectedErrSubstring) {
					t.Errorf("Expected error to contain '%s', got '%s'", tt.expectedErrSubstring, err.Error())
				}
			}

			// Verify removed expectation
			if removed != tt.expectRemoved {
				t.Errorf("Expected removed=%v, got %v", tt.expectRemoved, removed)
			}
		})
	}
}

func TestRemoveKubemacpoolExclusionIdempotent(t *testing.T) {
	t.Parallel()

	// Create context with namespace but no initial label
	ctx := createTestContext(true, true, true, true, true, "")

	// Ensure plan has a UID for reference counting
	ctx.Plan.SetUID(types.UID("idempotent-test-uid"))

	// First apply exclusion to add this plan to the owners
	applied, err := EnsureKubemacpoolExclusion(ctx)
	if err != nil {
		t.Fatalf("Failed to ensure exclusion: %v", err)
	}
	if !applied {
		t.Fatalf("Expected EnsureKubemacpoolExclusion to apply exclusion")
	}

	// First removal should return true (removes label since this is the only owner)
	removed1, err1 := RemoveKubemacpoolExclusion(ctx)
	if err1 != nil {
		t.Fatalf("First removal failed: %v", err1)
	}
	if !removed1 {
		t.Errorf("First removal should return true")
	}

	// Verify the label is actually removed
	ns := &core.Namespace{}
	if err := ctx.Destination.Client.Get(context.TODO(), k8sclient.ObjectKey{Name: "test-namespace"}, ns); err != nil {
		t.Fatalf("failed to re-fetch test namespace: %v", err)
	}
	if _, has := ns.Labels[KubemacpoolIgnoreLabelKey]; has {
		t.Fatalf("expected kubemacpool exclusion label to be removed after first call")
	}

	// Second removal should return false (idempotent)
	removed2, err2 := RemoveKubemacpoolExclusion(ctx)
	if err2 != nil {
		t.Fatalf("Second removal failed: %v", err2)
	}
	if removed2 {
		t.Errorf("Second removal should return false (idempotent)")
	}

	// Verify the label is still absent after second call
	if err := ctx.Destination.Client.Get(context.TODO(), k8sclient.ObjectKey{Name: "test-namespace"}, ns); err != nil {
		t.Fatalf("failed to re-fetch test namespace after second call: %v", err)
	}
	if _, has := ns.Labels[KubemacpoolIgnoreLabelKey]; has {
		t.Errorf("expected kubemacpool exclusion label to remain absent after second call")
	}

	// Third removal should also return false
	removed3, err3 := RemoveKubemacpoolExclusion(ctx)
	if err3 != nil {
		t.Fatalf("Third removal failed: %v", err3)
	}
	if removed3 {
		t.Errorf("Third removal should return false (idempotent)")
	}

	// Verify the label is still absent after third call
	if err := ctx.Destination.Client.Get(context.TODO(), k8sclient.ObjectKey{Name: "test-namespace"}, ns); err != nil {
		t.Fatalf("failed to re-fetch test namespace after third call: %v", err)
	}
	if _, has := ns.Labels[KubemacpoolIgnoreLabelKey]; has {
		t.Errorf("expected kubemacpool exclusion label to remain absent after third call")
	}
}

func createTestContext(sourceIsOCP, sourceIsHost, destIsOCP, destIsHost, namespaceExists bool, existingLabel string) *plancontext.Context {
	// Create mock providers
	var sourceType, destType v1beta1.ProviderType
	if sourceIsOCP {
		sourceType = v1beta1.OpenShift
	} else {
		sourceType = v1beta1.VSphere
	}
	if destIsOCP {
		destType = v1beta1.OpenShift
	} else {
		destType = v1beta1.VSphere
	}

	// Create mock providers without adding them to the fake client
	sourceProvider := &v1beta1.Provider{
		Spec: v1beta1.ProviderSpec{
			Type: &sourceType,
		},
	}
	if sourceIsOCP && sourceIsHost {
		sourceProvider.Spec.URL = "" // Host provider
	} else if sourceIsOCP {
		sourceProvider.Spec.URL = "https://remote-source.example.com"
	}

	destProvider := &v1beta1.Provider{
		Spec: v1beta1.ProviderSpec{
			Type: &destType,
		},
	}
	if destIsOCP && destIsHost {
		destProvider.Spec.URL = "" // Host provider
	} else if destIsOCP {
		destProvider.Spec.URL = "https://remote-dest.example.com"
	}

	// Create plan
	plan := &v1beta1.Plan{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-plan",
			UID:  "test-plan-uid",
		},
		Spec: v1beta1.PlanSpec{
			TargetNamespace: "test-namespace",
		},
	}

	// Set up referenced providers directly (skip the fake client for Provider objects)
	plan.Referenced.Provider.Source = sourceProvider
	plan.Referenced.Provider.Destination = destProvider

	// Create namespace (conditionally) - only add to client the objects that will be queried
	var objects []runtime.Object
	if namespaceExists {
		namespace := &core.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-namespace",
			},
		}
		if existingLabel != "" {
			namespace.Labels = map[string]string{
				KubemacpoolIgnoreLabelKey: existingLabel,
			}
		}
		objects = append(objects, namespace)
	}

	// Create fake client with minimal scheme (just core types)
	testScheme := runtime.NewScheme()
	if err := scheme.AddToScheme(testScheme); err != nil {
		panic(err) // This should never fail in tests
	}
	fakeClient := fake.NewClientBuilder().
		WithScheme(testScheme).
		WithRuntimeObjects(objects...).
		Build()

	// Create context
	ctx := &plancontext.Context{
		Client: fakeClient,
		Plan:   plan,
		Log:    logging.WithName("test"),
	}

	// Set up destination client
	ctx.Destination.Client = fakeClient

	return ctx
}

func TestKubemacpool_LabelPreservation(t *testing.T) {
	t.Parallel()

	// Start with a namespace that has an unrelated label
	ctx := createTestContext(true, true, true, true, true, "")

	// Ensure plan has a UID for reference counting
	ctx.Plan.SetUID(types.UID("preservation-test-uid"))

	ns := &core.Namespace{}
	if err := ctx.Destination.Client.Get(context.TODO(), k8sclient.ObjectKey{Name: "test-namespace"}, ns); err != nil {
		t.Fatalf("failed to fetch test namespace: %v", err)
	}
	if ns.Labels == nil {
		ns.Labels = map[string]string{}
	}
	ns.Labels["keep"] = "me"
	if err := ctx.Destination.Client.Update(context.TODO(), ns); err != nil {
		t.Fatalf("failed to seed extra label: %v", err)
	}

	// Ensure exclusion adds only the kubemacpool label
	applied, err := EnsureKubemacpoolExclusion(ctx)
	if err != nil {
		t.Fatalf("EnsureKubemacpoolExclusion failed: %v", err)
	}
	if !applied {
		t.Fatalf("expected applied=true")
	}
	if err := ctx.Destination.Client.Get(context.TODO(), k8sclient.ObjectKey{Name: "test-namespace"}, ns); err != nil {
		t.Fatalf("failed to re-fetch namespace: %v", err)
	}
	if ns.Labels["keep"] != "me" {
		t.Errorf("expected unrelated label to be preserved; got %q", ns.Labels["keep"])
	}
	if ns.Labels[KubemacpoolIgnoreLabelKey] != KubemacpoolIgnoreLabelValue {
		t.Errorf("expected kubemacpool label to be present")
	}

	// Remove exclusion removes only the kubemacpool label
	removed, err := RemoveKubemacpoolExclusion(ctx)
	if err != nil {
		t.Fatalf("RemoveKubemacpoolExclusion failed: %v", err)
	}
	if !removed {
		t.Fatalf("expected removed=true")
	}
	if err := ctx.Destination.Client.Get(context.TODO(), k8sclient.ObjectKey{Name: "test-namespace"}, ns); err != nil {
		t.Fatalf("failed to re-fetch namespace after removal: %v", err)
	}
	if ns.Labels["keep"] != "me" {
		t.Errorf("expected unrelated label to be preserved after removal; got %q", ns.Labels["keep"])
	}
	if _, has := ns.Labels[KubemacpoolIgnoreLabelKey]; has {
		t.Errorf("expected kubemacpool label to be removed")
	}
}

// Tests for concurrent plan scenarios with reference counting

func TestKubemacpool_ConcurrentPlans(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		description string
		test        func(*testing.T)
	}{
		{
			name:        "two plans same namespace",
			description: "Two plans targeting the same namespace should coordinate properly",
			test:        testTwoPlansOneNamespace,
		},
		{
			name:        "three plans overlapping lifecycle",
			description: "Three plans with overlapping lifecycles should maintain label correctly",
			test:        testThreePlansOverlapping,
		},
		{
			name:        "plan restart scenario",
			description: "Plan restarting should handle duplicate additions gracefully",
			test:        testPlanRestart,
		},
		{
			name:        "orphaned annotation cleanup",
			description: "Removing non-existent plan should be graceful",
			test:        testOrphanedAnnotation,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.test(t)
		})
	}
}

func testTwoPlansOneNamespace(t *testing.T) {
	// Create two separate plan contexts for the same namespace
	ctx1 := createTestContext(true, true, true, true, true, "")  // Plan 1
	ctx2 := createTestContext(true, true, true, true, false, "") // Plan 2 (namespace already exists)

	// Give them different UIDs
	ctx1.Plan.SetUID(types.UID("plan-1-uid"))
	ctx2.Plan.SetUID(types.UID("plan-2-uid"))

	// Both should use the same client and namespace
	ctx2.Destination.Client = ctx1.Destination.Client
	ctx2.Plan.Spec.TargetNamespace = ctx1.Plan.Spec.TargetNamespace

	// Plan 1 applies exclusion first
	applied1, err := EnsureKubemacpoolExclusion(ctx1)
	if err != nil {
		t.Fatalf("Plan 1 EnsureKubemacpoolExclusion failed: %v", err)
	}
	if !applied1 {
		t.Fatalf("Plan 1 should have applied exclusion")
	}

	// Verify namespace state after plan 1
	namespace := &core.Namespace{}
	err = ctx1.Destination.Client.Get(context.TODO(), k8sclient.ObjectKey{Name: ctx1.Plan.Spec.TargetNamespace}, namespace)
	if err != nil {
		t.Fatalf("Failed to get namespace: %v", err)
	}

	if namespace.Labels[KubemacpoolIgnoreLabelKey] != KubemacpoolIgnoreLabelValue {
		t.Errorf("Expected kubemacpool label to be set")
	}

	owners := getPlanOwners(namespace)
	if len(owners) != 1 || owners[0] != "plan-1-uid" {
		t.Errorf("Expected owners to contain only plan-1-uid, got %v", owners)
	}

	// Plan 2 applies exclusion to same namespace
	applied2, err := EnsureKubemacpoolExclusion(ctx2)
	if err != nil {
		t.Fatalf("Plan 2 EnsureKubemacpoolExclusion failed: %v", err)
	}
	if !applied2 {
		t.Fatalf("Plan 2 should have applied exclusion")
	}

	// Verify namespace state after plan 2
	err = ctx1.Destination.Client.Get(context.TODO(), k8sclient.ObjectKey{Name: ctx1.Plan.Spec.TargetNamespace}, namespace)
	if err != nil {
		t.Fatalf("Failed to get namespace: %v", err)
	}

	if namespace.Labels[KubemacpoolIgnoreLabelKey] != KubemacpoolIgnoreLabelValue {
		t.Errorf("Expected kubemacpool label to still be set")
	}

	owners = getPlanOwners(namespace)
	if len(owners) != 2 {
		t.Errorf("Expected 2 owners, got %d: %v", len(owners), owners)
	}

	// Verify both plans are in owners
	foundPlan1, foundPlan2 := false, false
	for _, owner := range owners {
		if owner == "plan-1-uid" {
			foundPlan1 = true
		}
		if owner == "plan-2-uid" {
			foundPlan2 = true
		}
	}
	if !foundPlan1 || !foundPlan2 {
		t.Errorf("Expected both plans in owners, got %v", owners)
	}

	// Plan 1 completes and removes exclusion
	removed1, err := RemoveKubemacpoolExclusion(ctx1)
	if err != nil {
		t.Fatalf("Plan 1 RemoveKubemacpoolExclusion failed: %v", err)
	}
	if removed1 {
		t.Errorf("Plan 1 should not have removed label (plan 2 still running)")
	}

	// Verify namespace state after plan 1 removal
	err = ctx1.Destination.Client.Get(context.TODO(), k8sclient.ObjectKey{Name: ctx1.Plan.Spec.TargetNamespace}, namespace)
	if err != nil {
		t.Fatalf("Failed to get namespace: %v", err)
	}

	if namespace.Labels[KubemacpoolIgnoreLabelKey] != KubemacpoolIgnoreLabelValue {
		t.Errorf("Expected kubemacpool label to still be set (plan 2 running)")
	}

	owners = getPlanOwners(namespace)
	if len(owners) != 1 || owners[0] != "plan-2-uid" {
		t.Errorf("Expected owners to contain only plan-2-uid, got %v", owners)
	}

	// Plan 2 completes and removes exclusion
	removed2, err := RemoveKubemacpoolExclusion(ctx2)
	if err != nil {
		t.Fatalf("Plan 2 RemoveKubemacpoolExclusion failed: %v", err)
	}
	if !removed2 {
		t.Errorf("Plan 2 should have removed label (last plan)")
	}

	// Verify namespace state after plan 2 removal
	err = ctx1.Destination.Client.Get(context.TODO(), k8sclient.ObjectKey{Name: ctx1.Plan.Spec.TargetNamespace}, namespace)
	if err != nil {
		t.Fatalf("Failed to get namespace: %v", err)
	}

	if _, exists := namespace.Labels[KubemacpoolIgnoreLabelKey]; exists {
		t.Errorf("Expected kubemacpool label to be removed")
	}

	owners = getPlanOwners(namespace)
	if len(owners) != 0 {
		t.Errorf("Expected no owners, got %v", owners)
	}
}

func testThreePlansOverlapping(t *testing.T) {
	// Create three plan contexts
	plans := make([]*plancontext.Context, 3)
	for i := 0; i < 3; i++ {
		// Only first plan creates the namespace
		namespaceExists := i == 0
		plans[i] = createTestContext(true, true, true, true, namespaceExists, "")
		plans[i].Plan.SetUID(types.UID(fmt.Sprintf("plan-%d-uid", i+1)))
		if i > 0 {
			plans[i].Destination.Client = plans[0].Destination.Client
			plans[i].Plan.Spec.TargetNamespace = plans[0].Plan.Spec.TargetNamespace
		}
	}

	namespace := &core.Namespace{}

	// Plan 1 starts
	_, err := EnsureKubemacpoolExclusion(plans[0])
	if err != nil {
		t.Fatalf("Plan 1 failed: %v", err)
	}

	// Plan 2 starts
	_, err = EnsureKubemacpoolExclusion(plans[1])
	if err != nil {
		t.Fatalf("Plan 2 failed: %v", err)
	}

	// Plan 3 starts
	_, err = EnsureKubemacpoolExclusion(plans[2])
	if err != nil {
		t.Fatalf("Plan 3 failed: %v", err)
	}

	// Verify all three plans are tracked
	err = plans[0].Destination.Client.Get(context.TODO(), k8sclient.ObjectKey{Name: plans[0].Plan.Spec.TargetNamespace}, namespace)
	if err != nil {
		t.Fatalf("Failed to get namespace: %v", err)
	}

	owners := getPlanOwners(namespace)
	if len(owners) != 3 {
		t.Errorf("Expected 3 owners, got %d: %v", len(owners), owners)
	}

	// Plan 2 completes (middle one)
	removed, err := RemoveKubemacpoolExclusion(plans[1])
	if err != nil {
		t.Fatalf("Plan 2 removal failed: %v", err)
	}
	if removed {
		t.Errorf("Plan 2 should not have removed label (other plans still running)")
	}

	// Verify label still exists and two plans remain
	err = plans[0].Destination.Client.Get(context.TODO(), k8sclient.ObjectKey{Name: plans[0].Plan.Spec.TargetNamespace}, namespace)
	if err != nil {
		t.Fatalf("Failed to get namespace: %v", err)
	}

	if namespace.Labels[KubemacpoolIgnoreLabelKey] != KubemacpoolIgnoreLabelValue {
		t.Errorf("Expected kubemacpool label to still be set")
	}

	owners = getPlanOwners(namespace)
	if len(owners) != 2 {
		t.Errorf("Expected 2 owners after plan 2 removal, got %d: %v", len(owners), owners)
	}

	// Plan 1 and 3 complete
	_, err = RemoveKubemacpoolExclusion(plans[0])
	if err != nil {
		t.Fatalf("Plan 1 removal failed: %v", err)
	}

	removed, err = RemoveKubemacpoolExclusion(plans[2])
	if err != nil {
		t.Fatalf("Plan 3 removal failed: %v", err)
	}
	if !removed {
		t.Errorf("Plan 3 should have removed label (last plan)")
	}

	// Verify cleanup
	err = plans[0].Destination.Client.Get(context.TODO(), k8sclient.ObjectKey{Name: plans[0].Plan.Spec.TargetNamespace}, namespace)
	if err != nil {
		t.Fatalf("Failed to get namespace: %v", err)
	}

	if _, exists := namespace.Labels[KubemacpoolIgnoreLabelKey]; exists {
		t.Errorf("Expected kubemacpool label to be removed")
	}

	owners = getPlanOwners(namespace)
	if len(owners) != 0 {
		t.Errorf("Expected no owners, got %v", owners)
	}
}

func testPlanRestart(t *testing.T) {
	ctx := createTestContext(true, true, true, true, true, "")
	ctx.Plan.SetUID(types.UID("restart-plan-uid"))

	// Plan applies exclusion
	_, err := EnsureKubemacpoolExclusion(ctx)
	if err != nil {
		t.Fatalf("Initial EnsureKubemacpoolExclusion failed: %v", err)
	}

	// Plan "restarts" and applies exclusion again (should be idempotent)
	applied, err := EnsureKubemacpoolExclusion(ctx)
	if err != nil {
		t.Fatalf("Restart EnsureKubemacpoolExclusion failed: %v", err)
	}
	if !applied {
		t.Errorf("Restart should still report applied=true")
	}

	// Verify only one entry in owners
	namespace := &core.Namespace{}
	err = ctx.Destination.Client.Get(context.TODO(), k8sclient.ObjectKey{Name: ctx.Plan.Spec.TargetNamespace}, namespace)
	if err != nil {
		t.Fatalf("Failed to get namespace: %v", err)
	}

	owners := getPlanOwners(namespace)
	if len(owners) != 1 || owners[0] != "restart-plan-uid" {
		t.Errorf("Expected single owner 'restart-plan-uid', got %v", owners)
	}

	// Plan completes
	removed, err := RemoveKubemacpoolExclusion(ctx)
	if err != nil {
		t.Fatalf("RemoveKubemacpoolExclusion failed: %v", err)
	}
	if !removed {
		t.Errorf("Should have removed label")
	}

	// Verify cleanup
	err = ctx.Destination.Client.Get(context.TODO(), k8sclient.ObjectKey{Name: ctx.Plan.Spec.TargetNamespace}, namespace)
	if err != nil {
		t.Fatalf("Failed to get namespace: %v", err)
	}

	if _, exists := namespace.Labels[KubemacpoolIgnoreLabelKey]; exists {
		t.Errorf("Expected kubemacpool label to be removed")
	}
}

func testOrphanedAnnotation(t *testing.T) {
	ctx := createTestContext(true, true, true, true, true, "")
	ctx.Plan.SetUID(types.UID("orphan-plan-uid"))

	// Manually create namespace with orphaned annotation
	namespace := &core.Namespace{}
	err := ctx.Destination.Client.Get(context.TODO(), k8sclient.ObjectKey{Name: ctx.Plan.Spec.TargetNamespace}, namespace)
	if err != nil {
		t.Fatalf("Failed to get namespace: %v", err)
	}

	if namespace.Annotations == nil {
		namespace.Annotations = make(map[string]string)
	}
	namespace.Annotations[KubemacpoolOwnersAnnotationKey] = "some-other-plan-uid"

	err = ctx.Destination.Client.Update(context.TODO(), namespace)
	if err != nil {
		t.Fatalf("Failed to create orphaned annotation: %v", err)
	}

	// Try to remove non-existent plan
	removed, err := RemoveKubemacpoolExclusion(ctx)
	if err != nil {
		t.Fatalf("RemoveKubemacpoolExclusion should handle orphaned annotation gracefully: %v", err)
	}
	if removed {
		t.Errorf("Should not have removed label (plan was not in owners)")
	}

	// Verify orphaned annotation is unchanged
	err = ctx.Destination.Client.Get(context.TODO(), k8sclient.ObjectKey{Name: ctx.Plan.Spec.TargetNamespace}, namespace)
	if err != nil {
		t.Fatalf("Failed to get namespace: %v", err)
	}

	owners := getPlanOwners(namespace)
	if len(owners) != 1 || owners[0] != "some-other-plan-uid" {
		t.Errorf("Expected orphaned annotation to be preserved, got %v", owners)
	}
}

func TestEnsureKubemacpoolExclusion_URLNormalization(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		sourceURL     string
		destURL       string
		expectApplied bool
		expectError   bool
	}{
		// Same remote cluster - URL normalization cases
		{
			name:          "Same cluster: default port vs no port",
			sourceURL:     "https://api.cluster.com:443",
			destURL:       "https://api.cluster.com",
			expectApplied: true,
			expectError:   false,
		},
		{
			name:          "Same cluster: mixed case normalization",
			sourceURL:     "HTTPS://API.CLUSTER.COM",
			destURL:       "https://api.cluster.com",
			expectApplied: true,
			expectError:   false,
		},
		{
			name:          "Same cluster: trailing slash vs none",
			sourceURL:     "https://api.cluster.com/",
			destURL:       "https://api.cluster.com",
			expectApplied: true,
			expectError:   false,
		},
		{
			name:          "Same cluster: complex normalization",
			sourceURL:     "HTTPS://API.CLUSTER.COM:443/",
			destURL:       "https://api.cluster.com",
			expectApplied: true,
			expectError:   false,
		},
		// Different clusters - non-default ports should NOT match
		{
			name:          "Different clusters: same host, different non-default ports",
			sourceURL:     "https://api.example.com:6443",
			destURL:       "https://api.example.com:8443",
			expectApplied: false, // Cross-cluster, should skip
			expectError:   false,
		},
		{
			name:          "Different clusters: different hosts",
			sourceURL:     "https://api.cluster1.com:6443",
			destURL:       "https://api.cluster2.com:6443",
			expectApplied: false, // Cross-cluster, should skip
			expectError:   false,
		},
		// Same cluster with non-default ports
		{
			name:          "Same cluster: identical non-default ports",
			sourceURL:     "https://api.cluster.com:6443",
			destURL:       "https://api.cluster.com:6443",
			expectApplied: true,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create test context with custom URLs
			ctx := createTestContextWithURLs(tt.sourceURL, tt.destURL)

			// Set a UID for the plan for reference counting
			ctx.Plan.SetUID(types.UID("url-test-" + strings.ReplaceAll(tt.name, " ", "-")))

			// Execute
			applied, err := EnsureKubemacpoolExclusion(ctx)

			// Verify function result
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if applied != tt.expectApplied {
				t.Errorf("Expected applied=%v, got applied=%v", tt.expectApplied, applied)
				t.Errorf("  Source URL: %q", tt.sourceURL)
				t.Errorf("  Dest URL:   %q", tt.destURL)
			}

			// Additional verification for applied cases
			if tt.expectApplied && applied {
				// Verify the namespace has the kubemacpool ignore label
				namespace := &core.Namespace{}
				err := ctx.Destination.Client.Get(context.TODO(), k8sclient.ObjectKey{Name: ctx.Plan.Spec.TargetNamespace}, namespace)
				if err != nil {
					t.Fatalf("Failed to get namespace: %v", err)
				}

				if namespace.Labels[KubemacpoolIgnoreLabelKey] != KubemacpoolIgnoreLabelValue {
					t.Errorf("Expected kubemacpool ignore label to be set")
				}

				// Verify the managed annotation is set
				if namespace.Annotations[KubemacpoolManagedAnnotationKey] != "true" {
					t.Errorf("Expected kubemacpool managed annotation to be set")
				}
			}
		})
	}
}

// createTestContextWithURLs creates a test context with custom URLs for testing URL normalization
func createTestContextWithURLs(sourceURL, destURL string) *plancontext.Context {
	// Create OpenShift providers with custom URLs
	sourceProvider := &v1beta1.Provider{
		Spec: v1beta1.ProviderSpec{
			Type: &[]v1beta1.ProviderType{v1beta1.OpenShift}[0],
			URL:  sourceURL,
		},
	}

	destProvider := &v1beta1.Provider{
		Spec: v1beta1.ProviderSpec{
			Type: &[]v1beta1.ProviderType{v1beta1.OpenShift}[0],
			URL:  destURL,
		},
	}

	// Create plan
	plan := &v1beta1.Plan{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-plan",
			UID:  "test-plan-uid",
		},
		Spec: v1beta1.PlanSpec{
			TargetNamespace: "test-namespace",
		},
	}

	// Set up referenced providers directly
	plan.Referenced.Provider.Source = sourceProvider
	plan.Referenced.Provider.Destination = destProvider

	// Create namespace
	namespace := &core.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-namespace",
		},
	}

	// Set up fake client with namespace
	objects := []runtime.Object{namespace}
	testScheme := runtime.NewScheme()
	if err := core.AddToScheme(testScheme); err != nil {
		panic(fmt.Sprintf("failed to add core/v1 to scheme: %v", err))
	}
	if err := v1beta1.SchemeBuilder.AddToScheme(testScheme); err != nil {
		panic(fmt.Sprintf("failed to add forklift/v1beta1 to scheme: %v", err))
	}
	fakeClient := fake.NewClientBuilder().WithScheme(testScheme).WithRuntimeObjects(objects...).Build()

	// Create context
	return &plancontext.Context{
		Plan: plan,
		Destination: plancontext.Destination{
			Client: fakeClient,
		},
		Log: logging.WithName("test"),
	}
}

func TestNormalizeURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Basic normalization
		{
			name:     "empty URL",
			input:    "",
			expected: "",
		},
		{
			name:     "simple HTTP",
			input:    "http://example.com",
			expected: "http://example.com",
		},
		{
			name:     "simple HTTPS",
			input:    "https://example.com",
			expected: "https://example.com",
		},
		// Trailing slash removal
		{
			name:     "HTTP with trailing slash",
			input:    "http://example.com/",
			expected: "http://example.com",
		},
		{
			name:     "HTTPS with trailing slash",
			input:    "https://api.cluster.com/",
			expected: "https://api.cluster.com",
		},
		{
			name:     "URL with path and trailing slash",
			input:    "https://api.cluster.com/api/v1/",
			expected: "https://api.cluster.com/api/v1",
		},
		// Default port removal
		{
			name:     "HTTP with default port 80",
			input:    "http://example.com:80",
			expected: "http://example.com",
		},
		{
			name:     "HTTPS with default port 443",
			input:    "https://api.cluster.com:443",
			expected: "https://api.cluster.com",
		},
		{
			name:     "HTTP with default port and trailing slash",
			input:    "http://example.com:80/",
			expected: "http://example.com",
		},
		{
			name:     "HTTPS with default port and trailing slash",
			input:    "https://api.cluster.com:443/",
			expected: "https://api.cluster.com",
		},
		// Non-default port preservation
		{
			name:     "HTTP with non-default port",
			input:    "http://example.com:8080",
			expected: "http://example.com:8080",
		},
		{
			name:     "HTTPS with OCP API port 6443",
			input:    "https://api.cluster.com:6443",
			expected: "https://api.cluster.com:6443",
		},
		{
			name:     "HTTPS with alternative port 8443",
			input:    "https://api.cluster.com:8443",
			expected: "https://api.cluster.com:8443",
		},
		{
			name:     "HTTPS with custom port and path",
			input:    "https://api.cluster.com:6443/api/v1",
			expected: "https://api.cluster.com:6443/api/v1",
		},
		// Case normalization
		{
			name:     "uppercase scheme",
			input:    "HTTPS://API.CLUSTER.COM",
			expected: "https://api.cluster.com",
		},
		{
			name:     "mixed case host",
			input:    "https://API.Cluster.COM:6443",
			expected: "https://api.cluster.com:6443",
		},
		{
			name:     "mixed case with default port",
			input:    "HTTPS://API.CLUSTER.COM:443/",
			expected: "https://api.cluster.com",
		},
		// Non-HTTP schemes
		{
			name:     "custom scheme with port",
			input:    "custom://example.com:9999",
			expected: "custom://example.com:9999",
		},
		// Invalid URL fallback
		{
			name:     "malformed URL",
			input:    "not-a-url",
			expected: "not-a-url",
		},
		{
			name:     "URL with trailing slash fallback",
			input:    "malformed://url/",
			expected: "malformed://url",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeURL(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeURL(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestArchiveCleanup_MultipleOwners(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                    string
		initialOwners           []string
		planToRemove            string
		expectedOwnersAfter     []string
		expectLabelRemoved      bool
		expectAnnotationRemoved bool
	}{
		{
			name:                    "Remove plan from multiple owners - label should remain",
			initialOwners:           []string{"plan-1-uid", "plan-2-uid", "plan-3-uid"},
			planToRemove:            "plan-2-uid",
			expectedOwnersAfter:     []string{"plan-1-uid", "plan-3-uid"},
			expectLabelRemoved:      false,
			expectAnnotationRemoved: false,
		},
		{
			name:                    "Remove second-to-last plan - label should remain",
			initialOwners:           []string{"plan-1-uid", "plan-2-uid"},
			planToRemove:            "plan-1-uid",
			expectedOwnersAfter:     []string{"plan-2-uid"},
			expectLabelRemoved:      false,
			expectAnnotationRemoved: false,
		},
		{
			name:                    "Remove last plan - label and annotation should be removed",
			initialOwners:           []string{"plan-1-uid"},
			planToRemove:            "plan-1-uid",
			expectedOwnersAfter:     []string{},
			expectLabelRemoved:      true,
			expectAnnotationRemoved: true,
		},
		{
			name:                    "Remove non-existent plan - no changes",
			initialOwners:           []string{"plan-1-uid", "plan-2-uid"},
			planToRemove:            "plan-3-uid",
			expectedOwnersAfter:     []string{"plan-1-uid", "plan-2-uid"},
			expectLabelRemoved:      false,
			expectAnnotationRemoved: false,
		},
		{
			name:                    "Remove from empty owners - no changes",
			initialOwners:           []string{},
			planToRemove:            "plan-1-uid",
			expectedOwnersAfter:     []string{},
			expectLabelRemoved:      false,
			expectAnnotationRemoved: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create test namespace with initial kubemacpool label and owners
			namespace := &core.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "archive-test-ns",
					Labels: map[string]string{
						KubemacpoolIgnoreLabelKey: KubemacpoolIgnoreLabelValue,
					},
					Annotations: map[string]string{
						KubemacpoolManagedAnnotationKey: "true",
					},
				},
			}

			// Set initial owners if any
			if len(tt.initialOwners) > 0 {
				namespace.Annotations[KubemacpoolOwnersAnnotationKey] = strings.Join(tt.initialOwners, ",")
			}

			// Create fake client with the namespace
			scheme := runtime.NewScheme()
			_ = core.AddToScheme(scheme)
			_ = v1beta1.SchemeBuilder.AddToScheme(scheme)
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(namespace).Build()

			// Create context for the plan to be removed
			ctx := &plancontext.Context{
				Plan: &v1beta1.Plan{
					ObjectMeta: metav1.ObjectMeta{
						Name: "archive-test-plan",
						UID:  types.UID(tt.planToRemove),
					},
					Spec: v1beta1.PlanSpec{
						TargetNamespace: "archive-test-ns",
					},
				},
				Destination: plancontext.Destination{
					Client: fakeClient,
				},
				Log: logging.WithName("archive-test"),
			}

			// Set up provider references for same-cluster OCP migration
			sourceProvider := &v1beta1.Provider{
				Spec: v1beta1.ProviderSpec{
					Type: &[]v1beta1.ProviderType{v1beta1.OpenShift}[0],
					URL:  "", // Host provider
				},
			}
			destProvider := &v1beta1.Provider{
				Spec: v1beta1.ProviderSpec{
					Type: &[]v1beta1.ProviderType{v1beta1.OpenShift}[0],
					URL:  "", // Host provider
				},
			}
			ctx.Plan.Referenced.Provider.Source = sourceProvider
			ctx.Plan.Referenced.Provider.Destination = destProvider

			// Execute removal (simulating Archive() call)
			removed, err := RemoveKubemacpoolExclusion(ctx)

			// Verify no error
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Verify label removal expectation
			if removed != tt.expectLabelRemoved {
				t.Errorf("Expected removed=%v, got removed=%v", tt.expectLabelRemoved, removed)
			}

			// Verify final namespace state
			finalNamespace := &core.Namespace{}
			err = fakeClient.Get(context.TODO(), k8sclient.ObjectKey{Name: "archive-test-ns"}, finalNamespace)
			if err != nil {
				t.Fatalf("Failed to get namespace after removal: %v", err)
			}

			// Check label state
			_, labelExists := finalNamespace.Labels[KubemacpoolIgnoreLabelKey]
			if tt.expectLabelRemoved && labelExists {
				t.Errorf("Expected kubemacpool label to be removed but it still exists")
			}
			if !tt.expectLabelRemoved && !labelExists {
				t.Errorf("Expected kubemacpool label to remain but it was removed")
			}

			// Check managed annotation state
			managedValue, managedExists := finalNamespace.Annotations[KubemacpoolManagedAnnotationKey]
			if tt.expectAnnotationRemoved && managedExists {
				t.Errorf("Expected managed annotation to be removed but it still exists: %v", managedValue)
			}
			if !tt.expectAnnotationRemoved && (!managedExists || managedValue != "true") {
				t.Errorf("Expected managed annotation to remain but it was removed or incorrect")
			}

			// Check owners annotation state
			finalOwners := getPlanOwners(finalNamespace)
			if len(finalOwners) != len(tt.expectedOwnersAfter) {
				t.Errorf("Expected %d owners after removal, got %d", len(tt.expectedOwnersAfter), len(finalOwners))
			}

			// Verify exact owner list (order independent)
			expectedSet := make(map[string]bool)
			for _, owner := range tt.expectedOwnersAfter {
				expectedSet[owner] = true
			}
			for _, owner := range finalOwners {
				if !expectedSet[owner] {
					t.Errorf("Unexpected owner in final list: %s", owner)
				}
				delete(expectedSet, owner)
			}
			for owner := range expectedSet {
				t.Errorf("Missing expected owner in final list: %s", owner)
			}

			t.Logf("Archive cleanup test passed: removed=%v, finalOwners=%v", removed, finalOwners)
		})
	}
}

func TestArchiveCleanup_PreExistingLabel(t *testing.T) {
	t.Parallel()

	// Test that Archive() cleanup preserves pre-existing kubemacpool labels
	// (ones not applied by Forklift)

	// Create namespace with kubemacpool label but NO managed annotation
	// (simulating user manually applied the label)
	namespace := &core.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "preexisting-test-ns",
			Labels: map[string]string{
				KubemacpoolIgnoreLabelKey: KubemacpoolIgnoreLabelValue,
			},
			Annotations: map[string]string{
				KubemacpoolOwnersAnnotationKey: "plan-1-uid", // Single owner
				// NO KubemacpoolManagedAnnotationKey - simulates pre-existing label
			},
		},
	}

	// Create fake client
	scheme := runtime.NewScheme()
	_ = core.AddToScheme(scheme)
	_ = v1beta1.SchemeBuilder.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(namespace).Build()

	// Create context for the last remaining plan
	ctx := &plancontext.Context{
		Plan: &v1beta1.Plan{
			ObjectMeta: metav1.ObjectMeta{
				Name: "preexisting-test-plan",
				UID:  types.UID("plan-1-uid"),
			},
			Spec: v1beta1.PlanSpec{
				TargetNamespace: "preexisting-test-ns",
			},
		},
		Destination: plancontext.Destination{
			Client: fakeClient,
		},
		Log: logging.WithName("preexisting-test"),
	}

	// Set up provider references for same-cluster OCP migration
	sourceProvider := &v1beta1.Provider{
		Spec: v1beta1.ProviderSpec{
			Type: &[]v1beta1.ProviderType{v1beta1.OpenShift}[0],
			URL:  "", // Host provider
		},
	}
	destProvider := &v1beta1.Provider{
		Spec: v1beta1.ProviderSpec{
			Type: &[]v1beta1.ProviderType{v1beta1.OpenShift}[0],
			URL:  "", // Host provider
		},
	}
	ctx.Plan.Referenced.Provider.Source = sourceProvider
	ctx.Plan.Referenced.Provider.Destination = destProvider

	// Execute removal (simulating Archive() call for last plan)
	removed, err := RemoveKubemacpoolExclusion(ctx)

	// Verify no error
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should not remove pre-existing label
	if removed {
		t.Errorf("Expected removed=false for pre-existing label, got removed=true")
	}

	// Verify final namespace state
	finalNamespace := &core.Namespace{}
	err = fakeClient.Get(context.TODO(), k8sclient.ObjectKey{Name: "preexisting-test-ns"}, finalNamespace)
	if err != nil {
		t.Fatalf("Failed to get namespace after removal: %v", err)
	}

	// Pre-existing label should be preserved
	if finalNamespace.Labels[KubemacpoolIgnoreLabelKey] != KubemacpoolIgnoreLabelValue {
		t.Errorf("Expected pre-existing kubemacpool label to be preserved")
	}

	// Owners annotation should be removed (last plan gone)
	if _, exists := finalNamespace.Annotations[KubemacpoolOwnersAnnotationKey]; exists {
		t.Errorf("Expected owners annotation to be removed when last plan is gone")
	}

	// Managed annotation should not exist (wasn't there originally)
	if _, exists := finalNamespace.Annotations[KubemacpoolManagedAnnotationKey]; exists {
		t.Errorf("Expected managed annotation to remain absent for pre-existing label")
	}

	t.Logf("Pre-existing label preservation test passed")
}

func TestCancelCleanup_ConcurrentOwners(t *testing.T) {
	t.Parallel()

	// Test that cancelling a plan with concurrent owners retains the kubemacpool label
	// This is the key scenario mentioned in the user's request

	tests := []struct {
		name                    string
		initialOwners           []string
		planToCancel            string
		expectedOwnersAfter     []string
		expectLabelRemoved      bool
		expectAnnotationRemoved bool
	}{
		{
			name:                    "Cancel plan with concurrent owner - label should remain",
			initialOwners:           []string{"plan-1-uid", "plan-2-uid"},
			planToCancel:            "plan-1-uid",
			expectedOwnersAfter:     []string{"plan-2-uid"},
			expectLabelRemoved:      false,
			expectAnnotationRemoved: false,
		},
		{
			name:                    "Cancel plan with multiple concurrent owners - label should remain",
			initialOwners:           []string{"plan-1-uid", "plan-2-uid", "plan-3-uid"},
			planToCancel:            "plan-2-uid",
			expectedOwnersAfter:     []string{"plan-1-uid", "plan-3-uid"},
			expectLabelRemoved:      false,
			expectAnnotationRemoved: false,
		},
		{
			name:                    "Cancel last remaining plan - label should be removed",
			initialOwners:           []string{"plan-1-uid"},
			planToCancel:            "plan-1-uid",
			expectedOwnersAfter:     []string{},
			expectLabelRemoved:      true,
			expectAnnotationRemoved: true,
		},
		{
			name:                    "Cancel non-existent plan - no changes",
			initialOwners:           []string{"plan-1-uid", "plan-2-uid"},
			planToCancel:            "plan-3-uid",
			expectedOwnersAfter:     []string{"plan-1-uid", "plan-2-uid"},
			expectLabelRemoved:      false,
			expectAnnotationRemoved: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create test namespace with initial kubemacpool label and owners
			namespace := &core.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cancel-test-ns",
					Labels: map[string]string{
						KubemacpoolIgnoreLabelKey: KubemacpoolIgnoreLabelValue,
					},
					Annotations: map[string]string{
						KubemacpoolManagedAnnotationKey: "true",
					},
				},
			}

			// Set initial owners if any
			if len(tt.initialOwners) > 0 {
				namespace.Annotations[KubemacpoolOwnersAnnotationKey] = strings.Join(tt.initialOwners, ",")
			}

			// Create fake client with the namespace
			scheme := runtime.NewScheme()
			_ = core.AddToScheme(scheme)
			_ = v1beta1.SchemeBuilder.AddToScheme(scheme)
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(namespace).Build()

			// Create context for the plan to be canceled
			ctx := &plancontext.Context{
				Plan: &v1beta1.Plan{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cancel-test-plan",
						UID:  types.UID(tt.planToCancel),
					},
					Spec: v1beta1.PlanSpec{
						TargetNamespace: "cancel-test-ns",
					},
				},
				Destination: plancontext.Destination{
					Client: fakeClient,
				},
				Log: logging.WithName("cancel-test"),
			}

			// Set up provider references for same-cluster OCP migration
			sourceProvider := &v1beta1.Provider{
				Spec: v1beta1.ProviderSpec{
					Type: &[]v1beta1.ProviderType{v1beta1.OpenShift}[0],
					URL:  "", // Host provider
				},
			}
			destProvider := &v1beta1.Provider{
				Spec: v1beta1.ProviderSpec{
					Type: &[]v1beta1.ProviderType{v1beta1.OpenShift}[0],
					URL:  "", // Host provider
				},
			}
			ctx.Plan.Referenced.Provider.Source = sourceProvider
			ctx.Plan.Referenced.Provider.Destination = destProvider

			// Execute removal (simulating Cancel() call)
			removed, err := RemoveKubemacpoolExclusion(ctx)

			// Verify no error
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Verify label removal expectation
			if removed != tt.expectLabelRemoved {
				t.Errorf("Expected removed=%v, got removed=%v", tt.expectLabelRemoved, removed)
			}

			// Verify final namespace state
			finalNamespace := &core.Namespace{}
			err = fakeClient.Get(context.TODO(), k8sclient.ObjectKey{Name: "cancel-test-ns"}, finalNamespace)
			if err != nil {
				t.Fatalf("Failed to get namespace after cancellation: %v", err)
			}

			// Check label state - critical for concurrent owners test
			_, labelExists := finalNamespace.Labels[KubemacpoolIgnoreLabelKey]
			if tt.expectLabelRemoved && labelExists {
				t.Errorf("Expected kubemacpool label to be removed but it still exists")
			}
			if !tt.expectLabelRemoved && !labelExists {
				t.Errorf("Expected kubemacpool label to remain (concurrent owners) but it was removed")
			}

			// Check managed annotation state
			managedValue, managedExists := finalNamespace.Annotations[KubemacpoolManagedAnnotationKey]
			if tt.expectAnnotationRemoved && managedExists {
				t.Errorf("Expected managed annotation to be removed but it still exists: %v", managedValue)
			}
			if !tt.expectAnnotationRemoved && (!managedExists || managedValue != "true") {
				t.Errorf("Expected managed annotation to remain but it was removed or incorrect")
			}

			// Check owners annotation state (critical test)
			finalOwners := getPlanOwners(finalNamespace)
			if len(finalOwners) != len(tt.expectedOwnersAfter) {
				t.Errorf("Expected %d owners after cancellation, got %d", len(tt.expectedOwnersAfter), len(finalOwners))
			}

			// Verify exact owner list (order independent)
			expectedSet := make(map[string]bool)
			for _, owner := range tt.expectedOwnersAfter {
				expectedSet[owner] = true
			}
			for _, owner := range finalOwners {
				if !expectedSet[owner] {
					t.Errorf("Unexpected owner in final list: %s", owner)
				}
				delete(expectedSet, owner)
			}
			for owner := range expectedSet {
				t.Errorf("Missing expected owner in final list: %s", owner)
			}

			t.Logf("Cancel cleanup test passed: removed=%v, finalOwners=%v", removed, finalOwners)
		})
	}
}

func TestCancelCleanup_IdempotentBehavior(t *testing.T) {
	t.Parallel()

	// Test that cancellation cleanup is idempotent - multiple calls don't break anything

	// Create namespace with kubemacpool label and single owner
	namespace := &core.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "idempotent-test-ns",
			Labels: map[string]string{
				KubemacpoolIgnoreLabelKey: KubemacpoolIgnoreLabelValue,
			},
			Annotations: map[string]string{
				KubemacpoolManagedAnnotationKey: "true",
				KubemacpoolOwnersAnnotationKey:  "plan-1-uid",
			},
		},
	}

	// Create fake client
	scheme := runtime.NewScheme()
	_ = core.AddToScheme(scheme)
	_ = v1beta1.SchemeBuilder.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(namespace).Build()

	// Create context for the plan
	ctx := &plancontext.Context{
		Plan: &v1beta1.Plan{
			ObjectMeta: metav1.ObjectMeta{
				Name: "idempotent-test-plan",
				UID:  types.UID("plan-1-uid"),
			},
			Spec: v1beta1.PlanSpec{
				TargetNamespace: "idempotent-test-ns",
			},
		},
		Destination: plancontext.Destination{
			Client: fakeClient,
		},
		Log: logging.WithName("idempotent-test"),
	}

	// Set up provider references for same-cluster OCP migration
	sourceProvider := &v1beta1.Provider{
		Spec: v1beta1.ProviderSpec{
			Type: &[]v1beta1.ProviderType{v1beta1.OpenShift}[0],
			URL:  "", // Host provider
		},
	}
	destProvider := &v1beta1.Provider{
		Spec: v1beta1.ProviderSpec{
			Type: &[]v1beta1.ProviderType{v1beta1.OpenShift}[0],
			URL:  "", // Host provider
		},
	}
	ctx.Plan.Referenced.Provider.Source = sourceProvider
	ctx.Plan.Referenced.Provider.Destination = destProvider

	// First cancellation - should remove label and annotation
	removed1, err1 := RemoveKubemacpoolExclusion(ctx)
	if err1 != nil {
		t.Fatalf("Unexpected error on first cancellation: %v", err1)
	}
	if !removed1 {
		t.Errorf("Expected first cancellation to remove label")
	}

	// Second cancellation - should be idempotent (no error, no changes)
	removed2, err2 := RemoveKubemacpoolExclusion(ctx)
	if err2 != nil {
		t.Fatalf("Unexpected error on second cancellation: %v", err2)
	}
	if removed2 {
		t.Errorf("Expected second cancellation to be idempotent (no removal)")
	}

	// Third cancellation - should still be idempotent
	removed3, err3 := RemoveKubemacpoolExclusion(ctx)
	if err3 != nil {
		t.Fatalf("Unexpected error on third cancellation: %v", err3)
	}
	if removed3 {
		t.Errorf("Expected third cancellation to be idempotent (no removal)")
	}

	// Verify final state
	finalNamespace := &core.Namespace{}
	err := fakeClient.Get(context.TODO(), k8sclient.ObjectKey{Name: "idempotent-test-ns"}, finalNamespace)
	if err != nil {
		t.Fatalf("Failed to get namespace after idempotent test: %v", err)
	}

	// Label should be gone
	if _, exists := finalNamespace.Labels[KubemacpoolIgnoreLabelKey]; exists {
		t.Errorf("Expected kubemacpool label to be removed after cancellation")
	}

	// Managed annotation should be gone
	if _, exists := finalNamespace.Annotations[KubemacpoolManagedAnnotationKey]; exists {
		t.Errorf("Expected managed annotation to be removed after cancellation")
	}

	// Owners annotation should be gone
	if _, exists := finalNamespace.Annotations[KubemacpoolOwnersAnnotationKey]; exists {
		t.Errorf("Expected owners annotation to be removed after cancellation")
	}

	t.Logf("Idempotent cancellation test passed")
}

func TestEndOfMigrationCleanup_AllOutcomes(t *testing.T) {
	t.Parallel()

	// Test that end-of-migration cleanup works correctly for all outcomes:
	// success, failure, and cancellation

	tests := []struct {
		name                    string
		migrationOutcome        string // "success", "failure", "cancellation"
		initialOwners           []string
		planToComplete          string
		expectedOwnersAfter     []string
		expectLabelRemoved      bool
		expectAnnotationRemoved bool
	}{
		{
			name:                    "Successful migration with concurrent owners - label retained",
			migrationOutcome:        "success",
			initialOwners:           []string{"plan-1-uid", "plan-2-uid"},
			planToComplete:          "plan-1-uid",
			expectedOwnersAfter:     []string{"plan-2-uid"},
			expectLabelRemoved:      false,
			expectAnnotationRemoved: false,
		},
		{
			name:                    "Failed migration with concurrent owners - label retained",
			migrationOutcome:        "failure",
			initialOwners:           []string{"plan-1-uid", "plan-2-uid"},
			planToComplete:          "plan-1-uid",
			expectedOwnersAfter:     []string{"plan-2-uid"},
			expectLabelRemoved:      false,
			expectAnnotationRemoved: false,
		},
		{
			name:                    "Canceled migration with concurrent owners - label retained",
			migrationOutcome:        "cancellation",
			initialOwners:           []string{"plan-1-uid", "plan-2-uid"},
			planToComplete:          "plan-1-uid",
			expectedOwnersAfter:     []string{"plan-2-uid"},
			expectLabelRemoved:      false,
			expectAnnotationRemoved: false,
		},
		{
			name:                    "Successful migration last owner - label removed",
			migrationOutcome:        "success",
			initialOwners:           []string{"plan-1-uid"},
			planToComplete:          "plan-1-uid",
			expectedOwnersAfter:     []string{},
			expectLabelRemoved:      true,
			expectAnnotationRemoved: true,
		},
		{
			name:                    "Failed migration last owner - label removed",
			migrationOutcome:        "failure",
			initialOwners:           []string{"plan-1-uid"},
			planToComplete:          "plan-1-uid",
			expectedOwnersAfter:     []string{},
			expectLabelRemoved:      true,
			expectAnnotationRemoved: true,
		},
		{
			name:                    "Canceled migration last owner - label removed",
			migrationOutcome:        "cancellation",
			initialOwners:           []string{"plan-1-uid"},
			planToComplete:          "plan-1-uid",
			expectedOwnersAfter:     []string{},
			expectLabelRemoved:      true,
			expectAnnotationRemoved: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create test namespace with initial kubemacpool label and owners
			namespace := &core.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "end-migration-test-ns",
					Labels: map[string]string{
						KubemacpoolIgnoreLabelKey: KubemacpoolIgnoreLabelValue,
					},
					Annotations: map[string]string{
						KubemacpoolManagedAnnotationKey: "true",
					},
				},
			}

			// Set initial owners if any
			if len(tt.initialOwners) > 0 {
				namespace.Annotations[KubemacpoolOwnersAnnotationKey] = strings.Join(tt.initialOwners, ",")
			}

			// Create fake client with the namespace
			scheme := runtime.NewScheme()
			_ = core.AddToScheme(scheme)
			_ = v1beta1.SchemeBuilder.AddToScheme(scheme)
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(namespace).Build()

			// Create context for the plan that completed
			ctx := &plancontext.Context{
				Plan: &v1beta1.Plan{
					ObjectMeta: metav1.ObjectMeta{
						Name: "end-migration-test-plan",
						UID:  types.UID(tt.planToComplete),
					},
					Spec: v1beta1.PlanSpec{
						TargetNamespace: "end-migration-test-ns",
					},
				},
				Destination: plancontext.Destination{
					Client: fakeClient,
				},
				Log: logging.WithName("end-migration-test"),
			}

			// Set up provider references for same-cluster OCP migration
			sourceProvider := &v1beta1.Provider{
				Spec: v1beta1.ProviderSpec{
					Type: &[]v1beta1.ProviderType{v1beta1.OpenShift}[0],
					URL:  "", // Host provider
				},
			}
			destProvider := &v1beta1.Provider{
				Spec: v1beta1.ProviderSpec{
					Type: &[]v1beta1.ProviderType{v1beta1.OpenShift}[0],
					URL:  "", // Host provider
				},
			}
			ctx.Plan.Referenced.Provider.Source = sourceProvider
			ctx.Plan.Referenced.Provider.Destination = destProvider

			// Execute removal (simulating end() method call regardless of outcome)
			removed, err := RemoveKubemacpoolExclusion(ctx)

			// Verify no error
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Verify label removal expectation
			if removed != tt.expectLabelRemoved {
				t.Errorf("Expected removed=%v, got removed=%v (outcome: %s)", tt.expectLabelRemoved, removed, tt.migrationOutcome)
			}

			// Verify final namespace state
			finalNamespace := &core.Namespace{}
			err = fakeClient.Get(context.TODO(), k8sclient.ObjectKey{Name: "end-migration-test-ns"}, finalNamespace)
			if err != nil {
				t.Fatalf("Failed to get namespace after migration completion: %v", err)
			}

			// Check label state - critical for all outcomes
			_, labelExists := finalNamespace.Labels[KubemacpoolIgnoreLabelKey]
			if tt.expectLabelRemoved && labelExists {
				t.Errorf("Expected kubemacpool label to be removed after %s but it still exists", tt.migrationOutcome)
			}
			if !tt.expectLabelRemoved && !labelExists {
				t.Errorf("Expected kubemacpool label to remain after %s (concurrent owners) but it was removed", tt.migrationOutcome)
			}

			// Check managed annotation state
			managedValue, managedExists := finalNamespace.Annotations[KubemacpoolManagedAnnotationKey]
			if tt.expectAnnotationRemoved && managedExists {
				t.Errorf("Expected managed annotation to be removed after %s but it still exists: %v", tt.migrationOutcome, managedValue)
			}
			if !tt.expectAnnotationRemoved && (!managedExists || managedValue != "true") {
				t.Errorf("Expected managed annotation to remain after %s but it was removed or incorrect", tt.migrationOutcome)
			}

			// Check owners annotation state
			finalOwners := getPlanOwners(finalNamespace)
			if len(finalOwners) != len(tt.expectedOwnersAfter) {
				t.Errorf("Expected %d owners after %s, got %d", len(tt.expectedOwnersAfter), tt.migrationOutcome, len(finalOwners))
			}

			// Verify exact owner list (order independent)
			expectedSet := make(map[string]bool)
			for _, owner := range tt.expectedOwnersAfter {
				expectedSet[owner] = true
			}
			for _, owner := range finalOwners {
				if !expectedSet[owner] {
					t.Errorf("Unexpected owner in final list after %s: %s", tt.migrationOutcome, owner)
				}
				delete(expectedSet, owner)
			}
			for owner := range expectedSet {
				t.Errorf("Missing expected owner in final list after %s: %s", tt.migrationOutcome, owner)
			}

			t.Logf("End-of-migration cleanup test passed (%s): removed=%v, finalOwners=%v", tt.migrationOutcome, removed, finalOwners)
		})
	}
}

func TestEndOfMigrationCleanup_CrossClusterMigration(t *testing.T) {
	t.Parallel()

	// Test that end-of-migration cleanup correctly skips cross-cluster migrations
	// (regardless of outcome)

	outcomes := []string{"success", "failure", "cancellation"}

	for _, outcome := range outcomes {
		outcome := outcome
		t.Run("Cross-cluster "+outcome+" - no cleanup", func(t *testing.T) {
			t.Parallel()

			// Create namespace (should remain untouched)
			namespace := &core.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cross-cluster-test-ns",
					Labels: map[string]string{
						KubemacpoolIgnoreLabelKey: KubemacpoolIgnoreLabelValue,
					},
				},
			}

			// Create fake client
			scheme := runtime.NewScheme()
			_ = core.AddToScheme(scheme)
			_ = v1beta1.SchemeBuilder.AddToScheme(scheme)
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(namespace).Build()

			// Create context for cross-cluster migration
			ctx := &plancontext.Context{
				Plan: &v1beta1.Plan{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cross-cluster-test-plan",
						UID:  types.UID("cross-cluster-plan-uid"),
					},
					Spec: v1beta1.PlanSpec{
						TargetNamespace: "cross-cluster-test-ns",
					},
				},
				Destination: plancontext.Destination{
					Client: fakeClient,
				},
				Log: logging.WithName("cross-cluster-test"),
			}

			// Set up provider references for cross-cluster OCP migration
			sourceProvider := &v1beta1.Provider{
				Spec: v1beta1.ProviderSpec{
					Type: &[]v1beta1.ProviderType{v1beta1.OpenShift}[0],
					URL:  "", // Host provider (source cluster)
				},
			}
			destProvider := &v1beta1.Provider{
				Spec: v1beta1.ProviderSpec{
					Type: &[]v1beta1.ProviderType{v1beta1.OpenShift}[0],
					URL:  "https://remote-cluster.example.com", // Remote provider (different cluster)
				},
			}
			ctx.Plan.Referenced.Provider.Source = sourceProvider
			ctx.Plan.Referenced.Provider.Destination = destProvider

			// Execute removal (should skip cross-cluster migrations)
			removed, err := RemoveKubemacpoolExclusion(ctx)

			// Verify no error and no action taken
			if err != nil {
				t.Fatalf("Unexpected error for cross-cluster %s: %v", outcome, err)
			}
			if removed {
				t.Errorf("Expected no removal for cross-cluster %s, but got removed=true", outcome)
			}

			// Verify namespace remains untouched
			finalNamespace := &core.Namespace{}
			err = fakeClient.Get(context.TODO(), k8sclient.ObjectKey{Name: "cross-cluster-test-ns"}, finalNamespace)
			if err != nil {
				t.Fatalf("Failed to get namespace after cross-cluster %s: %v", outcome, err)
			}

			// Label should remain untouched
			if finalNamespace.Labels[KubemacpoolIgnoreLabelKey] != KubemacpoolIgnoreLabelValue {
				t.Errorf("Expected kubemacpool label to remain untouched for cross-cluster %s", outcome)
			}

			t.Logf("Cross-cluster cleanup test passed (%s): correctly skipped removal", outcome)
		})
	}
}

func TestIsSameClusterMigration(t *testing.T) {
	t.Parallel()

	// Helper to create providers
	createProvider := func(providerType v1beta1.ProviderType, url string) *v1beta1.Provider {
		return &v1beta1.Provider{
			Spec: v1beta1.ProviderSpec{
				Type: &providerType,
				URL:  url,
			},
		}
	}

	tests := []struct {
		name           string
		sourceProvider *v1beta1.Provider
		destProvider   *v1beta1.Provider
		expected       bool
	}{
		// Both host providers (local cluster)
		{
			name:           "both are host providers",
			sourceProvider: createProvider(v1beta1.OpenShift, ""),
			destProvider:   createProvider(v1beta1.OpenShift, ""),
			expected:       true,
		},
		// Same remote cluster - identical URLs
		{
			name:           "same remote cluster - identical URLs",
			sourceProvider: createProvider(v1beta1.OpenShift, "https://api.cluster.com:6443"),
			destProvider:   createProvider(v1beta1.OpenShift, "https://api.cluster.com:6443"),
			expected:       true,
		},
		// Same remote cluster - normalized URLs
		{
			name:           "same remote cluster - default port vs no port",
			sourceProvider: createProvider(v1beta1.OpenShift, "https://api.cluster.com:443"),
			destProvider:   createProvider(v1beta1.OpenShift, "https://api.cluster.com"),
			expected:       true,
		},
		{
			name:           "same remote cluster - case differences",
			sourceProvider: createProvider(v1beta1.OpenShift, "HTTPS://API.CLUSTER.COM"),
			destProvider:   createProvider(v1beta1.OpenShift, "https://api.cluster.com"),
			expected:       true,
		},
		{
			name:           "same remote cluster - trailing slash differences",
			sourceProvider: createProvider(v1beta1.OpenShift, "https://api.cluster.com/"),
			destProvider:   createProvider(v1beta1.OpenShift, "https://api.cluster.com"),
			expected:       true,
		},
		{
			name:           "same remote cluster - mixed normalizations",
			sourceProvider: createProvider(v1beta1.OpenShift, "HTTPS://API.CLUSTER.COM:443/"),
			destProvider:   createProvider(v1beta1.OpenShift, "https://api.cluster.com"),
			expected:       true,
		},
		// Different clusters - different hosts
		{
			name:           "different clusters - different hosts",
			sourceProvider: createProvider(v1beta1.OpenShift, "https://api.cluster1.com:6443"),
			destProvider:   createProvider(v1beta1.OpenShift, "https://api.cluster2.com:6443"),
			expected:       false,
		},
		// Different clusters - same host, different non-default ports
		{
			name:           "different clusters - same host, different ports",
			sourceProvider: createProvider(v1beta1.OpenShift, "https://api.example.com:6443"),
			destProvider:   createProvider(v1beta1.OpenShift, "https://api.example.com:8443"),
			expected:       false,
		},
		// Mixed case: one host, one remote
		{
			name:           "mixed case - source host, dest remote",
			sourceProvider: createProvider(v1beta1.OpenShift, ""),
			destProvider:   createProvider(v1beta1.OpenShift, "https://api.cluster.com:6443"),
			expected:       false,
		},
		{
			name:           "mixed case - source remote, dest host",
			sourceProvider: createProvider(v1beta1.OpenShift, "https://api.cluster.com:6443"),
			destProvider:   createProvider(v1beta1.OpenShift, ""),
			expected:       false,
		},
		// Non-OpenShift providers
		{
			name:           "source not OpenShift",
			sourceProvider: createProvider(v1beta1.VSphere, ""),
			destProvider:   createProvider(v1beta1.OpenShift, ""),
			expected:       false,
		},
		{
			name:           "destination not OpenShift",
			sourceProvider: createProvider(v1beta1.OpenShift, ""),
			destProvider:   createProvider(v1beta1.VSphere, ""),
			expected:       false,
		},
		{
			name:           "both not OpenShift",
			sourceProvider: createProvider(v1beta1.VSphere, "https://vcenter.com"),
			destProvider:   createProvider(v1beta1.OVirt, "https://ovirt.com"),
			expected:       false,
		},
		// Edge cases with empty URLs
		{
			name:           "source has URL, dest has empty URL",
			sourceProvider: createProvider(v1beta1.OpenShift, "https://api.cluster.com"),
			destProvider:   createProvider(v1beta1.OpenShift, ""),
			expected:       false,
		},
		{
			name:           "source has empty URL, dest has URL",
			sourceProvider: createProvider(v1beta1.OpenShift, ""),
			destProvider:   createProvider(v1beta1.OpenShift, "https://api.cluster.com"),
			expected:       false,
		},
		// Invalid URLs (should still work with fallback)
		{
			name:           "both have invalid but identical URLs",
			sourceProvider: createProvider(v1beta1.OpenShift, "not-a-url"),
			destProvider:   createProvider(v1beta1.OpenShift, "not-a-url"),
			expected:       true,
		},
		{
			name:           "both have invalid but different URLs",
			sourceProvider: createProvider(v1beta1.OpenShift, "not-a-url"),
			destProvider:   createProvider(v1beta1.OpenShift, "different-invalid"),
			expected:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsSameClusterMigration(tt.sourceProvider, tt.destProvider)
			if result != tt.expected {
				t.Errorf("IsSameClusterMigration() = %v, expected %v", result, tt.expected)
				t.Errorf("  Source: Type=%v, URL=%q", tt.sourceProvider.Type(), tt.sourceProvider.Spec.URL)
				t.Errorf("  Dest:   Type=%v, URL=%q", tt.destProvider.Type(), tt.destProvider.Spec.URL)
			}
		})
	}
}
