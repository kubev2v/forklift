# Forklift Testing Guidelines

## Unit Testing

### Test File Organization
```go
// controller_test.go
package plan

import (
    "testing"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    
    api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
)

func TestReconciler_validate(t *testing.T) {
    tests := []struct {
        name        string
        plan        *api.Plan
        expectError bool
        expectConditions []string
    }{
        {
            name: "valid plan",
            plan: validPlan(),
            expectError: false,
            expectConditions: []string{},
        },
        {
            name: "missing provider",
            plan: planWithoutProvider(),
            expectError: false,
            expectConditions: []string{ProviderNotSet},
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            r := &Reconciler{}
            err := r.validate(tt.plan)
            
            if tt.expectError {
                require.Error(t, err)
            } else {
                require.NoError(t, err)
            }
            
            for _, condType := range tt.expectConditions {
                assert.True(t, tt.plan.Status.HasCondition(condType))
            }
        })
    }
}
```

### Validation Testing Patterns

#### Testing Critical Conditions
```go
func TestValidation_BlockingCondition(t *testing.T) {
    plan := &api.Plan{
        Spec: api.PlanSpec{
            SkipGuestConversion: true, // Requires VDDK
        },
        Referenced: api.Referenced{
            Provider: api.ProviderPair{
                Source: &api.Provider{
                    Spec: api.ProviderSpec{
                        Type: api.VSphere,
                        Settings: map[string]string{
                            // No VDDK image set
                        },
                    },
                },
            },
        },
    }
    
    r := &Reconciler{}
    err := r.validateVddkImage(plan)
    require.NoError(t, err)
    
    // Should have critical condition
    condition := plan.Status.FindCondition(VDDKInitImageUnavailable)
    require.NotNil(t, condition)
    assert.Equal(t, api.CategoryCritical, condition.Category)
    assert.Equal(t, True, condition.Status)
}
```

#### Testing Provider-Specific Logic
```go
func TestValidation_ProviderSpecific(t *testing.T) {
    tests := []struct {
        name         string
        providerType api.ProviderType
        expectRun    bool
    }{
        {
            name:         "VMware provider",
            providerType: api.VSphere,
            expectRun:    true,
        },
        {
            name:         "oVirt provider", 
            providerType: api.Ovirt,
            expectRun:    false,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            plan := planWithProviderType(tt.providerType)
            
            r := &Reconciler{}
            err := r.validateVMwareSpecific(plan)
            require.NoError(t, err)
            
            // Check if validation ran based on provider type
            hasCondition := plan.Status.HasCondition(VMwareSpecificCondition)
            assert.Equal(t, tt.expectRun, hasCondition)
        })
    }
}
```

### Mock Patterns

#### Client Mocking
```go
type mockClient struct {
    client.Client
    getFunc func(ctx context.Context, key client.ObjectKey, obj client.Object) error
}

func (m *mockClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object) error {
    if m.getFunc != nil {
        return m.getFunc(ctx, key, obj)
    }
    return nil
}

func TestWithMockClient(t *testing.T) {
    mock := &mockClient{
        getFunc: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
            return k8serr.NewNotFound(schema.GroupResource{}, key.Name)
        },
    }
    
    r := &Reconciler{Client: mock}
    // Test with mocked client behavior
}
```

#### Provider Adapter Mocking
```go
type mockValidator struct {
    warmMigrationSupported bool
    changeTrackingEnabled  bool
}

func (m *mockValidator) WarmMigration() bool {
    return m.warmMigrationSupported
}

func (m *mockValidator) ChangeTrackingEnabled(ref ref.Ref) (bool, error) {
    return m.changeTrackingEnabled, nil
}

func TestWithMockValidator(t *testing.T) {
    validator := &mockValidator{
        warmMigrationSupported: false,
    }
    
    // Test validation logic with mock
}
```

## OPA Policy Testing

### Policy Test Structure
```rego
package io.konveyor.forklift.vmware_test

import future.keywords.in

test_btrfs_detected {
    result := btrfs_disks with input as {
        "guestDisks": [
            {
                "filesystemType": "BTRFS"
            }
        ]
    }
    
    count(result) == 1
    0 in result
}

test_btrfs_case_insensitive {
    result := btrfs_disks with input as {
        "guestDisks": [
            {
                "filesystemType": "btrfs"
            }
        ]
    }
    
    count(result) == 1
}

test_no_btrfs {
    result := btrfs_disks with input as {
        "guestDisks": [
            {
                "filesystemType": "ext4"
            }
        ]
    }
    
    count(result) == 0
}

test_concern_generated {
    result := concerns with input as {
        "guestDisks": [
            {
                "filesystemType": "BTRFS"
            }
        ]
    }
    
    count(result) == 1
    
    concern := result[_]
    concern.category == "Warning"
    contains(concern.label, "BTRFS")
}
```

### Policy Testing Best Practices
- Test positive and negative cases
- Test edge cases (empty input, missing fields)
- Test case sensitivity where applicable
- Verify concern structure and content

## Integration Testing

### Controller Integration Tests
```go
func TestController_Integration(t *testing.T) {
    // Setup test environment
    env := &envtest.Environment{
        CRDDirectoryPaths: []string{"../../config/crd/bases"},
    }
    
    cfg, err := env.Start()
    require.NoError(t, err)
    defer env.Stop()
    
    // Create client
    client, err := client.New(cfg, client.Options{})
    require.NoError(t, err)
    
    // Create test resources
    plan := &api.Plan{
        ObjectMeta: metav1.ObjectMeta{
            Name:      "test-plan",
            Namespace: "default",
        },
        Spec: api.PlanSpec{
            // Test configuration
        },
    }
    
    err = client.Create(context.TODO(), plan)
    require.NoError(t, err)
    
    // Test controller behavior
    reconciler := &Reconciler{Client: client}
    _, err = reconciler.Reconcile(context.TODO(), reconcile.Request{
        NamespacedName: types.NamespacedName{
            Name:      "test-plan",
            Namespace: "default",
        },
    })
    require.NoError(t, err)
    
    // Verify results
    updated := &api.Plan{}
    err = client.Get(context.TODO(), types.NamespacedName{
        Name: "test-plan", Namespace: "default"}, updated)
    require.NoError(t, err)
    
    assert.True(t, updated.Status.HasCondition(libcnd.Ready))
}
```

## Test Data Management

### Test Fixture Patterns
```go
func validPlan() *api.Plan {
    return &api.Plan{
        ObjectMeta: metav1.ObjectMeta{
            Name:      "valid-plan",
            Namespace: "default",
        },
        Spec: api.PlanSpec{
            Provider: api.ProviderPair{
                Source: core.ObjectReference{
                    Name:      "source-provider",
                    Namespace: "default",
                },
            },
            // Other valid configuration
        },
    }
}

func planWithVMwareProvider() *api.Plan {
    plan := validPlan()
    plan.Referenced.Provider.Source = &api.Provider{
        Spec: api.ProviderSpec{
            Type: api.VSphere,
            URL:  "https://vcenter.example.com",
        },
    }
    return plan
}
```

### Test Helper Functions
```go
func assertCondition(t *testing.T, plan *api.Plan, condType string, expectPresent bool) {
    t.Helper()
    hasCondition := plan.Status.HasCondition(condType)
    if expectPresent {
        assert.True(t, hasCondition, "Expected condition %s to be present", condType)
        condition := plan.Status.FindCondition(condType)
        assert.Equal(t, True, condition.Status)
    } else {
        assert.False(t, hasCondition, "Expected condition %s to be absent", condType)
    }
}

func assertCriticalCondition(t *testing.T, plan *api.Plan, condType string) {
    t.Helper()
    condition := plan.Status.FindCondition(condType)
    require.NotNil(t, condition, "Expected condition %s to exist", condType)
    assert.Equal(t, api.CategoryCritical, condition.Category)
    assert.Equal(t, True, condition.Status)
}
```

## Performance Testing

### Benchmark Tests
```go
func BenchmarkValidation(b *testing.B) {
    plan := validPlan()
    r := &Reconciler{}
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        plan.Status.Conditions = []libcnd.Condition{} // Reset
        err := r.validate(plan)
        if err != nil {
            b.Fatal(err)
        }
    }
}
```

### Memory Profiling
```go
func TestValidation_Memory(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping memory test in short mode")
    }
    
    plan := validPlan()
    r := &Reconciler{}
    
    // Measure memory usage
    var m1, m2 runtime.MemStats
    runtime.GC()
    runtime.ReadMemStats(&m1)
    
    for i := 0; i < 1000; i++ {
        err := r.validate(plan)
        require.NoError(t, err)
    }
    
    runtime.GC()
    runtime.ReadMemStats(&m2)
    
    allocated := m2.Alloc - m1.Alloc
    t.Logf("Memory allocated: %d bytes", allocated)
    
    // Assert reasonable memory usage
    assert.Less(t, allocated, uint64(1024*1024), "Memory usage too high")
}
```
