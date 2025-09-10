# Forklift Migration Workflows

## Migration Types

### Cold Migration
- VM is powered off during the entire migration process
- Simplest type, minimal requirements
- Longer downtime but higher reliability
- No special prerequisites

```go
// Cold migration detection
isColdMigration := plan.Spec.Type == api.MigrationCold || 
                  (plan.Spec.Type == "" && !plan.Spec.Warm)
```

### Warm Migration  
- Uses Change Block Tracking (CBT) for incremental sync
- Minimal downtime during final cutover
- Requires CBT enabled on source VMs
- VMware and oVirt only

```go
// Warm migration detection  
isWarmMigration := plan.Spec.Warm || plan.Spec.Type == api.MigrationWarm

// Requirements validation
if isWarmMigration {
    // Check CBT enabled
    // Check no pre-existing snapshots
    // Validate VDDK for VMware
}
```

### Live Migration
- VM stays running throughout migration
- Very minimal downtime
- Limited platform support
- Complex requirements

### Conversion-Only Migration
- Only performs guest conversion, no data copy
- Requires pre-existing PVCs with VM data
- Useful for prepared environments

```go
isConversionOnly := plan.Spec.Type == api.MigrationOnlyConversion
```

## Migration Phases

### 1. Planning Phase
- Plan validation and readiness checks
- Resource mapping verification
- Prerequisites validation

### 2. Preparation Phase  
- Create target resources (PVCs, networks)
- Setup migration infrastructure
- Validate source VM accessibility

### 3. Data Transfer Phase
- Copy VM disks to target storage
- Handle incremental syncs for warm migration
- Monitor transfer progress

### 4. Conversion Phase (V2V)
- Install VirtIO drivers (VMware)
- Modify guest OS for KubeVirt compatibility
- Update boot configuration

### 5. VM Creation Phase
- Create KubeVirt VirtualMachine resource
- Apply target configuration
- Prepare for startup

### 6. Cutover Phase
- Stop source VM (if running)
- Final data sync (warm migration)
- Start target VM

## Raw Copy Mode (SkipGuestConversion)

### Purpose
- Bypasses guest OS conversion
- Assumes target VM already has appropriate drivers
- Faster migration but requires preparation

### Requirements
```go
if plan.Spec.SkipGuestConversion {
    // VDDK required for VMware
    if source.Type() == api.VSphere && vddkImage == "" {
        // Set critical condition
    }
    
    // Source VM should have VirtIO drivers pre-installed
    // or useCompatibilityMode should be enabled
}
```

### Use Cases
- Pre-prepared VMs with VirtIO drivers
- Testing scenarios
- Specialized deployment workflows

## Provider-Specific Workflows

### VMware vSphere
```go
// Special requirements
- VDDK for warm migration and raw copy mode
- CBT for warm migration  
- No snapshots for warm migration
- Guest tools for optimal conversion

// Validation considerations
if source.Type() == api.VSphere {
    validateVddkImage(plan)
    validateChangeBlockTracking(plan) 
    validateSnapshots(plan)
    validateVMwareTools(plan)
}
```

### oVirt/RHV
```go
// Capabilities
- Warm migration support
- Direct storage access
- Agent-based operations

// Validation considerations  
if source.Type() == api.Ovirt {
    validateOvirtAgent(plan)
    validateStorageAccess(plan)
}
```

### OpenStack
```go
// Characteristics
- API-based operations
- Image-based VMs
- Network complexity

// Validation considerations
if source.Type() == api.OpenStack {
    validateOpenstackAccess(plan)
    validateImageFormats(plan)
    validateNetworkConfig(plan)
}
```

### OVA Files
```go
// Import characteristics
- Single file format
- No warm migration
- Pre-packaged VMs

// Validation considerations
if source.Type() == api.Ova {
    validateOvaFile(plan)
    validateImportCapability(plan)
}
```

## Migration State Management

### Plan Status Tracking
```go
type MigrationStatus struct {
    VMs        []VMStatus
    Snapshots  []Snapshot
}

type VMStatus struct {
    Phase      string
    Progress   int
    Conditions []libcnd.Condition
    Pipeline   []Step
}
```

### Progress Monitoring
```go
// Track migration progress
func (r *Reconciler) updateProgress(vm *plan.VMStatus, phase string, progress int) {
    vm.Phase = phase
    vm.Progress = progress
    vm.MarkProgress()
}

// Common phases
const (
    Initialize      = "Initialize"
    DiskAllocation  = "DiskAllocation" 
    DiskTransfer    = "DiskTransfer"
    ImageConversion = "ImageConversion"
    VMCreation      = "VirtualMachineCreation"
)
```

### Error Recovery
```go
// Handle migration failures
func (r *Reconciler) handleMigrationError(vm *plan.VMStatus, err error) {
    vm.SetCondition(libcnd.Condition{
        Type:     Failed,
        Status:   True,
        Category: api.CategoryError,
        Message:  err.Error(),
        Durable:  true,
    })
    
    // Cleanup resources
    r.cleanupMigrationResources(vm)
}

// Retry logic
func (r *Reconciler) shouldRetry(vm *plan.VMStatus) bool {
    failureCount := vm.GetFailureCount()
    return failureCount < maxRetries && !vm.HasCondition(Canceled)
}
```

## Network and Storage Mapping

### Network Mapping Validation
```go
func (r *Reconciler) validateNetworkMapping(plan *api.Plan) error {
    for _, vm := range plan.Spec.VMs {
        for _, nic := range vm.NICs {
            if nic.Network == "" {
                // Unmapped network
                plan.Status.SetCondition(networkMappingError)
            }
            
            // Validate target network exists
            targetNet, err := r.resolveTargetNetwork(nic.Network)
            if err != nil {
                return err
            }
            
            // Check network compatibility
            if !r.isNetworkCompatible(nic, targetNet) {
                plan.Status.SetCondition(networkIncompatible)
            }
        }
    }
    return nil
}
```

### Storage Mapping Validation  
```go
func (r *Reconciler) validateStorageMapping(plan *api.Plan) error {
    for _, vm := range plan.Spec.VMs {
        for _, disk := range vm.Disks {
            if disk.StorageClass == "" {
                // Unmapped storage
                plan.Status.SetCondition(storageMappingError)
            }
            
            // Validate storage class exists
            sc, err := r.getStorageClass(disk.StorageClass)
            if err != nil {
                return err
            }
            
            // Check capacity and features
            if !r.isStorageCompatible(disk, sc) {
                plan.Status.SetCondition(storageIncompatible)
            }
        }
    }
    return nil
}
```

## Migration Cleanup

### Successful Migration Cleanup
```go
func (r *Reconciler) cleanupSuccessfulMigration(vm *plan.VMStatus) {
    // Remove temporary resources
    r.cleanupTempPVCs(vm)
    r.cleanupMigrationPods(vm)
    r.cleanupSnapshots(vm)
    
    // Mark completion
    vm.SetCondition(libcnd.Condition{
        Type:     Succeeded,
        Status:   True,
        Category: api.CategoryAdvisory,
        Message:  "Migration completed successfully",
        Durable:  true,
    })
}
```

### Failed Migration Cleanup
```go
func (r *Reconciler) cleanupFailedMigration(vm *plan.VMStatus) {
    // Preserve some resources for debugging
    r.preserveMigrationLogs(vm)
    
    // Clean up others
    r.cleanupTempPVCs(vm)
    r.cleanupMigrationPods(vm)
    
    // Leave target VM for investigation if created
}
```

### Cancellation Handling
```go
func (r *Reconciler) handleCancellation(plan *api.Plan) {
    for _, vm := range plan.Status.Migration.VMs {
        if !vm.HasAnyCondition(Succeeded, Failed) {
            vm.SetCondition(libcnd.Condition{
                Type:     Canceled,
                Status:   True,
                Category: api.CategoryAdvisory,
                Message:  "Migration was canceled",
                Durable:  true,
            })
        }
    }
    
    // Stop active operations
    r.cancelActiveMigrations(plan)
}
```

## Performance Considerations

### Parallel Processing
```go
// Control concurrent migrations
const maxConcurrentMigrations = 3

func (r *Reconciler) scheduleVMMigrations(plan *api.Plan) {
    activeCount := r.getActiveMigrationCount(plan)
    if activeCount >= maxConcurrentMigrations {
        return // Wait for slots to free up
    }
    
    // Start next pending migration
    pending := r.getPendingVMs(plan)
    if len(pending) > 0 {
        r.startVMMigration(pending[0])
    }
}
```

### Resource Management
```go
// Monitor resource usage
func (r *Reconciler) checkResourceLimits(plan *api.Plan) bool {
    usage := r.getCurrentResourceUsage()
    limits := r.getMigrationLimits()
    
    return usage.CPU < limits.CPU && 
           usage.Memory < limits.Memory &&
           usage.Storage < limits.Storage
}
```

### Progress Reporting
```go
// Update migration progress
func (r *Reconciler) reportProgress(vm *plan.VMStatus) {
    pipeline := vm.Pipeline
    completed := 0
    
    for _, step := range pipeline {
        if step.HasCondition(Completed) {
            completed++
        }
    }
    
    vm.Progress = (completed * 100) / len(pipeline)
    vm.MarkProgress()
}
```
