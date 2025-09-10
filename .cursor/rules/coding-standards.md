# Forklift Coding Standards

## Go Language Standards

### Code Formatting
- **Use `gofmt`** for all Go code formatting
- **Use `goimports`** to organize imports automatically
- **Line length**: Aim for 120 characters maximum
- **Indentation**: Use tabs (Go standard)
- **Blank lines**: Use blank lines to separate logical sections

### Naming Conventions
```go
// Package names: lowercase, single word
package controller

// Constants: CamelCase starting with uppercase
const MaxRetryAttempts = 3
const DefaultTimeout = time.Minute * 5

// Variables: camelCase starting with lowercase
var migrationTimeout time.Duration
var retryBackoff = []time.Duration{...}

// Types: CamelCase starting with uppercase
type PlanController struct {}
type MigrationStatus struct {}

// Methods: CamelCase starting with uppercase (exported) or lowercase (unexported)
func (r *Reconciler) ValidatePlan() error         // Exported
func (r *Reconciler) parseVMReference() error     // Unexported

// Interfaces: Often end with -er
type Validator interface {}
type Migrator interface {}
```

### File and Directory Naming
```
pkg/controller/plan/       # lowercase, descriptive
validation.go             # lowercase with underscores if needed
vm_status.go              # compound names with underscores
plan_controller_test.go   # test files with _test suffix
```

## Go Code Guidelines

### Error Handling
- Always wrap errors with context using `liberr.Wrap(err, "context", value)`
- Use structured error information where helpful
- Don't ignore errors - handle them appropriately

```go
// Good
err = r.Client.Get(ctx, key, object)
if err != nil {
    return liberr.Wrap(err, "object", key)
}

// Bad  
r.Client.Get(ctx, key, object)
```

### Logging
- Use structured logging with the controller's logger
- Include relevant context in log messages
- Use appropriate log levels (Info, Error, V(1), V(2))

```go
// Good
r.Log.Info("Processing migration plan", "plan", plan.Name, "namespace", plan.Namespace)
r.Log.V(2).Info("Detailed debug info", "step", "validation")

// Bad
fmt.Printf("Processing plan %s", plan.Name)
```

### Conditions and Status
- Use the standard condition pattern for status reporting
- Critical conditions block execution, warnings don't
- Always provide clear, actionable messages

```go
plan.Status.SetCondition(libcnd.Condition{
    Type:     ConditionType,
    Status:   True,
    Category: api.CategoryCritical,
    Reason:   NotSet,
    Message:  "Clear description of the issue and how to fix it",
})
```

### Variable Naming
- Use descriptive names for complex operations
- Follow Go naming conventions (camelCase for private, PascalCase for public)
- Use meaningful abbreviations sparingly

### Function Organization
- Keep functions focused and single-purpose
- Use early returns to reduce nesting
- Group related functionality together

## File Organization

### Controller Structure
- Validation logic goes in `validation.go`
- Main reconciliation logic in `controller.go`
- Helper functions in separate, well-named files

### Package Imports
- Standard library first
- Third-party packages second  
- Project packages last
- Group with blank lines between categories

```go
import (
    "context"
    "fmt"

    "k8s.io/client-go/kubernetes"
    libcnd "github.com/kubev2v/forklift/pkg/lib/condition"

    api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
)
```

## Documentation

### Code Comments
- Document exported functions and types
- Explain complex business logic
- Keep comments up to date with code changes

### Commit Messages
- Use conventional commit format when possible
- Include context about why the change was made
- Reference issue numbers when applicable

## Performance Considerations

### Resource Usage
- Be mindful of memory allocations in hot paths
- Use appropriate data structures for the use case
- Consider caching for frequently accessed data

### Kubernetes API Usage
- Batch operations when possible
- Use appropriate list/watch patterns
- Respect API rate limits and resource quotas

## Advanced Go Patterns

### Interface Design
```go
// Keep interfaces small and focused
type Validator interface {
    Validate(ctx context.Context, obj client.Object) error
}

// Prefer composition over large interfaces
type ProviderValidator interface {
    Validator
    ValidateProvider(provider *api.Provider) error
}
```

### Context Usage
```go
// Always pass context as first parameter
func (r *Reconciler) processVM(ctx context.Context, vm *api.VM) error {
    // Use context for cancellation and timeouts
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
        // Continue processing
    }
}

// Create child contexts with timeout
ctx, cancel := context.WithTimeout(ctx, time.Minute*5)
defer cancel()
```

### Concurrency Patterns
```go
// Use sync.WaitGroup for coordinating goroutines
var wg sync.WaitGroup
for _, vm := range vms {
    wg.Add(1)
    go func(vm *api.VM) {
        defer wg.Done()
        r.processVM(ctx, vm)
    }(vm)
}
wg.Wait()

// Use channels for communication
resultCh := make(chan Result, len(vms))
errorCh := make(chan error, len(vms))
```

### Memory Management
```go
// Use pointers for large structs
func processLargeStruct(ls *LargeStruct) error {
    // Avoid copying large structs
}

// Clear slices when done
defer func() {
    items = items[:0] // Clear but keep capacity
}()

// Pool expensive objects
var bufferPool = sync.Pool{
    New: func() interface{} {
        return make([]byte, 1024)
    },
}
```

## Code Style Guidelines

### Function Design
```go
// Functions should do one thing well
func validateProviderConnection(provider *api.Provider) error {
    // Single responsibility
}

// Use early returns to reduce nesting
func processRequest(req Request) error {
    if req == nil {
        return errors.New("request is nil")
    }
    
    if !req.IsValid() {
        return errors.New("invalid request")
    }
    
    // Main logic here
    return nil
}

// Keep function parameters reasonable (max 3-4 parameters)
func createMigration(plan *api.Plan, provider *api.Provider, options MigrationOptions) (*api.Migration, error) {
    // If more parameters needed, use a struct
}
```

### Struct Design
```go
// Group related fields together
type MigrationConfig struct {
    // Basic settings
    Type        api.MigrationType
    Warm        bool
    
    // Advanced options
    Timeout     time.Duration
    Retries     int
    
    // Provider settings
    ProviderRef core.ObjectReference
    Settings    map[string]string
}

// Use embedding for composition
type VMStatus struct {
    libcnd.Conditions   // Embedded for condition management
    
    Phase    string
    Progress int
}
```

### Constants and Enums
```go
// Group related constants
const (
    // Migration phases
    PhaseInitialize     = "Initialize"
    PhaseDiskTransfer   = "DiskTransfer"
    PhaseConversion     = "ImageConversion"
    PhaseVMCreation     = "VirtualMachineCreation"
)

// Use typed constants for enum-like behavior
type MigrationType string

const (
    MigrationCold MigrationType = "cold"
    MigrationWarm MigrationType = "warm"
    MigrationLive MigrationType = "live"
)
```

## Testing Standards

### Test File Organization
```go
// test_file_test.go
package controller

import (
    "testing"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestValidatePlan(t *testing.T) {
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
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

### Test Helpers
```go
// Create helper functions for common test data
func validPlan() *api.Plan {
    return &api.Plan{
        ObjectMeta: metav1.ObjectMeta{
            Name:      "test-plan",
            Namespace: "default",
        },
        Spec: api.PlanSpec{
            // Valid configuration
        },
    }
}

// Use require for fatal assertions, assert for non-fatal
func TestProcessing(t *testing.T) {
    result, err := process()
    require.NoError(t, err)      // Test fails if error
    assert.Equal(t, expected, result) // Test continues if not equal
}
```

## Security Guidelines

### Input Validation
```go
// Validate all inputs
func processUserInput(input string) error {
    if input == "" {
        return errors.New("input cannot be empty")
    }
    
    if len(input) > maxInputLength {
        return errors.New("input too long")
    }
    
    // Sanitize input
    cleaned := sanitize(input)
    return processCleanInput(cleaned)
}
```

### Secret Handling
```go
// Don't log secrets
r.Log.Info("Processing provider", "name", provider.Name) // Good
r.Log.Info("Processing provider", "password", provider.Password) // BAD

// Clear sensitive data
defer func() {
    secret.Data = nil
}()
```

### Resource Limits
```go
// Implement timeouts
ctx, cancel := context.WithTimeout(ctx, time.Minute*5)
defer cancel()

// Limit resource usage
if len(items) > maxItems {
    return errors.New("too many items")
}
```
