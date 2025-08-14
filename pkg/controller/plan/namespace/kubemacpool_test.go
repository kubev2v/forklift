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
				ctx.Plan.SetUID("ensure-edge-nil-source")
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
				ctx.Plan.SetUID("ensure-edge-nil-dest")
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
				ctx.Plan.SetUID("ensure-edge-empty-ns")
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
				ctx.Plan.SetUID("remove-edge-nil-source")
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
				ctx.Plan.SetUID("remove-edge-nil-dest")
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
				ctx.Plan.SetUID("remove-edge-empty-ns")
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
				ctx.Plan.SetUID("remove-edge-nil-client")
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
	ctx.Plan.SetUID("idempotent-test-uid")

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
	ctx.Plan.SetUID("preservation-test-uid")

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
	ctx1.Plan.SetUID("plan-1-uid")
	ctx2.Plan.SetUID("plan-2-uid")

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
	ctx.Plan.SetUID("restart-plan-uid")

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
	ctx.Plan.SetUID("orphan-plan-uid")

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
