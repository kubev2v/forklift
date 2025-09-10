# Forklift Controller Patterns

## Controller Structure

### Standard Reconciler Pattern
```go
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (result reconcile.Result, err error) {
    // Setup logging
    r.Log = logging.WithName("controller", "resource", request.NamespacedName)
    r.Started()
    
    defer func() {
        result.RequeueAfter = r.Ended(result.RequeueAfter, err)
        err = nil // Don't return errors from reconcile
    }()

    // Fetch the resource
    resource := &api.ResourceType{}
    err = r.Get(ctx, request.NamespacedName, resource)
    if err != nil {
        if k8serr.IsNotFound(err) {
            r.Log.Info("Resource deleted.")
            err = nil
        }
        return
    }

    // Begin staging conditions
    resource.Status.BeginStagingConditions()

    // Perform validation and business logic
    err = r.validate(resource)
    if err != nil {
        return
    }

    // Set ready condition if no blockers
    if !resource.Status.HasBlockerCondition() {
        resource.Status.SetCondition(libcnd.Condition{
            Type:     libcnd.Ready,
            Status:   True,
            Category: api.CategoryRequired,
            Message:  "Resource is ready.",
        })
    }

    // End staging conditions  
    resource.Status.EndStagingConditions()

    // Update status
    err = r.updateStatus(resource)
    return
}
```

### Validation Method Pattern
```go
func (r *Reconciler) validate(resource *api.ResourceType) error {
    // Validate references
    err := r.validateReferences(resource)
    if err != nil {
        return err
    }

    // Validate configuration
    err = r.validateConfiguration(resource)
    if err != nil {
        return err
    }

    // Provider-specific validation
    err = r.validateProvider(resource)
    if err != nil {
        return err
    }

    return nil
}
```

## Plan Controller Specifics

### Migration Execution Pattern
```go
func (r *Reconciler) execute(plan *api.Plan) (reQ time.Duration, err error) {
    // Check for blocking conditions
    if plan.Status.HasBlockerCondition() || plan.Status.HasCondition(Archived) {
        reQ = base.SlowReQ
        return
    }

    // Setup migration context
    ctx, err := plancontext.New(r, plan, r.Log)
    if err != nil {
        return
    }

    // Find active migration
    migration, err := r.activeMigration(plan)
    if err != nil {
        return
    }

    if migration != nil {
        ctx.SetMigration(migration)
        // Process active migration
    } else {
        // Look for pending migrations
        pending, err := r.pendingMigrations(plan)
        if err != nil {
            return
        }
        if len(pending) > 0 {
            migration = pending[0]
            ctx.SetMigration(migration)
            // Start new migration
        }
    }

    // Execute migration
    runner := Migration{Context: ctx}
    reQ, err = runner.Run()
    return
}
```

### Provider Adapter Pattern
```go
func (r *Reconciler) validateProvider(plan *api.Plan) error {
    provider := plan.Referenced.Provider.Source
    if provider == nil {
        return liberr.New("source provider not set")
    }

    // Create provider adapter
    pAdapter, err := adapter.New(provider)
    if err != nil {
        return err
    }

    // Get validator
    validator, err := pAdapter.Validator(ctx)
    if err != nil {
        return err
    }

    // Perform provider-specific validation
    if !validator.SupportsFeature() {
        plan.Status.SetCondition(libcnd.Condition{
            Type:     FeatureNotSupported,
            Status:   True,
            Category: api.CategoryCritical,
            Message:  "Provider does not support required feature",
        })
    }

    return nil
}
```

## Status Management

### Condition Lifecycle
```go
// Begin staging - prepares for condition updates
resource.Status.BeginStagingConditions()

// Set conditions during processing
resource.Status.SetCondition(condition)

// End staging - finalizes condition updates
resource.Status.EndStagingConditions()
```

### Status Update Pattern
```go
func (r *Reconciler) updateStatus(resource *api.ResourceType) error {
    // Update the status subresource
    err := r.Status().Update(context.TODO(), resource)
    if err != nil {
        r.Log.Error(err, "Failed to update status")
        return liberr.Wrap(err)
    }
    return nil
}
```

## Resource References

### Reference Resolution Pattern
```go
type Referenced struct {
    Provider struct {
        Source      *api.Provider
        Destination *api.Provider
    }
    Map struct {
        Network *api.NetworkMap
        Storage *api.StorageMap
    }
    Secret *core.Secret
}

func (r *Reconciler) resolveReferences(plan *api.Plan) error {
    // Source provider
    if libref.RefSet(&plan.Spec.Provider.Source) {
        provider := &api.Provider{}
        err := r.Get(context.TODO(), plan.Spec.Provider.Source.Namespace, provider)
        if err != nil {
            return liberr.Wrap(err, "source provider", plan.Spec.Provider.Source)
        }
        plan.Referenced.Provider.Source = provider
    }

    // Continue for other references...
    return nil
}
```

### Reference Validation Pattern
```go
func (r *Reconciler) validateReferences(plan *api.Plan) error {
    // Check if required references are set
    if !libref.RefSet(&plan.Spec.Provider.Source) {
        plan.Status.SetCondition(libcnd.Condition{
            Type:     ProviderNotSet,
            Status:   True,
            Category: api.CategoryCritical,
            Message:  "Source provider reference is required",
        })
        return nil
    }

    // Check if referenced resources exist and are ready
    if plan.Referenced.Provider.Source == nil {
        plan.Status.SetCondition(libcnd.Condition{
            Type:     ProviderNotFound,
            Status:   True,
            Category: api.CategoryCritical,
            Message:  "Source provider not found",
        })
        return nil
    }

    if !plan.Referenced.Provider.Source.Status.HasCondition(libcnd.Ready) {
        plan.Status.SetCondition(libcnd.Condition{
            Type:     ProviderNotReady,
            Status:   True,
            Category: api.CategoryCritical,
            Message:  "Source provider is not ready",
        })
        return nil
    }

    return nil
}
```

## Error Handling

### Controller Error Handling
```go
// Don't return errors from main reconcile loop
defer func() {
    result.RequeueAfter = r.Ended(result.RequeueAfter, err)
    err = nil // Clear error to prevent crash-loop
}()

// Handle specific error types
if k8serr.IsNotFound(err) {
    // Resource was deleted, normal case
    return reconcile.Result{}, nil
}

if k8serr.IsConflict(err) {
    // Resource was updated, retry
    return reconcile.Result{Requeue: true}, nil
}
```

### Business Logic Error Handling
```go
// Set conditions for business logic errors
if businessError {
    resource.Status.SetCondition(errorCondition)
    return nil // Don't return error, just set condition
}

// Return errors for unexpected system issues
if systemError {
    return liberr.Wrap(err, "context")
}
```

## Requeue Patterns

### Standard Requeue Times
```go
const (
    FastReQ = time.Second * 5
    SlowReQ = time.Minute * 2
)

// Use appropriate requeue time
if needsFastRetry {
    return reconcile.Result{RequeueAfter: FastReQ}, nil
}

if needsSlowRetry {
    return reconcile.Result{RequeueAfter: SlowReQ}, nil
}
```

### Condition-Based Requeuing
```go
if resource.Status.HasBlockerCondition() {
    // Slow requeue for blocked resources
    return reconcile.Result{RequeueAfter: SlowReQ}, nil
}

if resource.Status.HasCondition(InProgress) {
    // Fast requeue for active operations  
    return reconcile.Result{RequeueAfter: FastReQ}, nil
}
```
