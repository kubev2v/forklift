# Forklift Validation Patterns

## Controller Validation

### Condition Categories
Use the appropriate category for each validation:

- **CategoryCritical**: Blocks migration execution completely
- **CategoryError**: Blocks Ready condition but allows reconciliation  
- **CategoryWarn**: Advisory only, doesn't block execution
- **CategoryRequired**: Required for Ready state
- **CategoryAdvisory**: Informational messages

### Validation Function Structure

```go
func (r *Reconciler) validateFeature(plan *api.Plan) error {
    // Early returns for non-applicable scenarios
    if !featureApplies(plan) {
        return nil
    }
    
    // Gather required data
    data, err := r.getRequiredData(plan)
    if err != nil {
        return err
    }
    
    // Perform validation checks
    if validationFails(data) {
        plan.Status.SetCondition(libcnd.Condition{
            Type:     ValidationConditionType,
            Status:   True,
            Category: api.CategoryCritical,
            Reason:   SpecificReason,
            Message:  "Clear, actionable error message",
        })
    }
    
    return nil
}
```

### Common Validation Patterns

#### Provider-Specific Validation
```go
// Only validate for specific providers
source := plan.Referenced.Provider.Source
if source.Type() != api.VSphere {
    return nil // Not applicable to this provider
}
```

#### Feature Requirement Validation
```go
// Check if required feature is available when needed
if plan.Spec.FeatureEnabled && !hasRequiredDependency() {
    plan.Status.SetCondition(libcnd.Condition{
        Type:     DependencyMissing,
        Status:   True,
        Category: api.CategoryCritical,
        Message:  "Feature X requires dependency Y to be configured",
    })
}
```

#### Resource Existence Validation
```go
// Validate referenced resources exist
resource, err := r.getResource(plan.Spec.ResourceRef)
if err != nil {
    if errors.Is(err, NotFoundError{}) {
        plan.Status.SetCondition(libcnd.Condition{
            Type:     ResourceNotFound,
            Status:   True,
            Category: api.CategoryCritical,
            Message:  fmt.Sprintf("Referenced resource %s not found", plan.Spec.ResourceRef),
        })
        return nil // Don't return error, just set condition
    }
    return err // Unexpected error, return it
}
```

## OPA Policy Validation

### Policy Structure
```rego
package io.konveyor.forklift.provider

# Policy rule definition
rule_name[result] {
    # Condition logic
    condition_met
    
    # Result structure
    result := {
        "category": "Warning",          # or "Critical"
        "label": "Short description",
        "assessment": "Detailed explanation with context"
    }
}
```

### Common OPA Patterns

#### Array/List Processing
```rego
# Check each item in an array
problematic_items[item] {
    some i
    item := input.items[i]
    has_problem(item)
}
```

#### String Matching
```rego
# Case-insensitive string matching
lower_value := lower(input.field)
contains(lower_value, "pattern")
```

#### Conditional Logic
```rego
# Multiple conditions
rule_applies {
    input.type == "specific_type"
    input.enabled == true
    count(input.items) > 0
}
```

## Validation Best Practices

### Message Quality
- **Be Specific**: Explain exactly what's wrong
- **Be Actionable**: Tell users how to fix the issue
- **Be Contextual**: Include relevant details (names, values, etc.)

```go
// Good
Message: "VDDK image not set on provider 'prod-vsphere'. Configure VDDK image in provider settings for warm migration support."

// Bad  
Message: "VDDK missing"
```

### Error Handling in Validation
- Don't fail validation due to transient errors
- Set appropriate conditions for different error types
- Return errors only for unexpected system issues

### Performance Considerations
- Cache expensive lookups when possible
- Avoid N+1 queries in validation loops
- Use early returns to skip unnecessary work

### Testing Validation Logic
- Test positive and negative cases
- Verify condition types and categories
- Test error handling scenarios
- Mock external dependencies appropriately

## Migration-Specific Patterns

### Warm Migration Validation
```go
isWarmMigration := plan.Spec.Warm || plan.Spec.Type == api.MigrationWarm
if isWarmMigration {
    // Warm migration specific checks
}
```

### Raw Copy Mode Validation
```go
if plan.Spec.SkipGuestConversion {
    // Raw copy mode specific requirements
}
```

### Provider Capability Checks
```go
pAdapter, err := adapter.New(provider)
if err != nil {
    return err
}
validator, err := pAdapter.Validator(ctx)
if err != nil {
    return err
}
if !validator.SupportsFeature() {
    // Set appropriate condition
}
```
