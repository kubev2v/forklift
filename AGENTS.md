# AI Agent Guide for Forklift

This document provides guidance for AI assistants (Claude, GitHub Copilot, Cursor, and other AI agents) working with the Forklift project.

## Project Overview

Forklift is a toolkit for migrating virtual machines from VMware, OVA, oVirt, and OpenStack to KubeVirt. It provides a comprehensive migration solution with support for:

- **Warm migrations** using Change Block Tracking/Incremental Backup
- **Guest conversions** via virt-v2v for VMware migrations  
- **Remote cluster migrations** for cross-cluster orchestration
- **VM validations** to identify migration issues before execution
- **Multiple source platforms** (VMware vSphere, oVirt, OpenStack, OVA)

## Architecture Overview

### Core Components

- **Controllers**: Located in `pkg/controller/` - handle Kubernetes resource reconciliation
- **Operators**: Located in `operator/` - manage Forklift operator deployment and configuration
- **Migration Engine**: Orchestrates VM migration workflows
- **Validation Engine**: Pre-migration validation using OPA policies
- **Adapters**: Provider-specific implementations for different source platforms

### Key Technologies

- **Go**: Primary programming language
- **Kubernetes**: Target platform and orchestration
- **Open Policy Agent (OPA)**: Policy validation framework
- **KubeVirt**: Kubernetes VM management
- **virt-v2v**: VM guest conversion tool
- **VDDK**: VMware Virtual Disk Development Kit for disk operations

## Directory Structure

```
.
├── cmd/                     # Command-line tools and entry points
├── docs/                    # Project documentation
├── hack/                    # Build and development scripts
├── operator/               # Operator deployment manifests
├── pkg/                    # Go source code
│   ├── apis/              # API definitions
│   ├── controller/        # Kubernetes controllers
│   └── lib/               # Shared libraries
├── tests/                  # Test suites
├── validation/            # OPA validation policies
└── vendor/                # Go module dependencies
```

## Development Guidelines

### Code Patterns

1. **Controller Pattern**: Use the standard Kubernetes controller-runtime pattern
2. **Error Handling**: Wrap errors with context using `liberr.Wrap()`
3. **Logging**: Use structured logging with `r.Log.Info()` patterns
4. **Conditions**: Use `libcnd.Condition` for status reporting
5. **Validation**: Critical conditions block execution, warnings don't

### Key Files to Understand

- `pkg/controller/plan/validation.go`: Migration plan validation logic
- `pkg/controller/plan/controller.go`: Plan controller reconciliation
- `pkg/apis/forklift/v1beta1/plan.go`: Plan API definitions
- `validation/policies/`: OPA validation policies

### Common Tasks

#### Adding New Validations

1. **Controller Validation**: Add to `pkg/controller/plan/validation.go`
   ```go
   if condition && problem {
       plan.Status.SetCondition(libcnd.Condition{
           Type:     ConditionType,
           Status:   True,
           Category: api.CategoryCritical, // or Warn
           Message:  "Description of the issue",
       })
   }
   ```

2. **OPA Policy Validation**: Create `.rego` files in `validation/policies/`
   ```rego
   package io.konveyor.forklift.vmware
   
   concerns[flag] {
       # condition logic
       flag := {
           "category": "Warning",
           "label": "Issue description",
           "assessment": "Detailed explanation"
       }
   }
   ```

#### Condition Categories

- `CategoryCritical`: Blocks migration execution
- `CategoryError`: Blocks Ready condition
- `CategoryWarn`: Advisory only, doesn't block
- `CategoryRequired`: Required for Ready condition
- `CategoryAdvisory`: Informational

### Migration Types

- **Cold Migration**: VM powered off during migration
- **Warm Migration**: Uses CBT for incremental sync, minimal downtime
- **Live Migration**: VM stays running (limited scenarios)
- **Raw Copy Mode**: `SkipGuestConversion=true`, requires VDDK for VMware

### Provider Adapters

Each source platform has an adapter in `pkg/controller/plan/adapter/`:
- `vsphere/`: VMware vSphere integration
- `ovirt/`: oVirt/RHV integration  
- `openstack/`: OpenStack integration
- `ova/`: OVA file imports
- `ocp/`: OpenShift/Kubernetes sources

## Testing Guidelines

### Unit Tests
- Follow Go testing conventions with `_test.go` files
- Mock external dependencies
- Focus on business logic validation

### Integration Tests
- Located in `tests/`
- Test end-to-end migration scenarios
- Validate controller behavior

### Validation Testing
- Test OPA policies in isolation
- Verify condition generation logic
- Ensure proper error handling

## Debugging Tips

1. **Check Conditions**: Always look at `plan.Status.Conditions` for errors
2. **Log Analysis**: Use structured logging to trace execution
3. **Validation Issues**: Check both controller and OPA policy validations
4. **Provider Issues**: Look at adapter-specific logic for source platform problems
5. **VDDK Problems**: Verify VDDK image configuration for VMware migrations

## AI Assistant Guidelines

### When Working with Forklift

1. **Understand Context**: Always consider the migration workflow and validation pipeline
2. **Respect Patterns**: Follow existing controller and validation patterns
3. **Check Dependencies**: Be aware of provider-specific requirements (e.g., VDDK for VMware)
4. **Validate Changes**: Ensure new code follows the condition and error handling patterns
5. **Consider Impact**: Migration validation changes can block user workflows

### Code Quality

- Use `make build-controller` to verify compilation
- Run linting with appropriate tools
- Ensure proper error wrapping and logging
- Follow Kubernetes API conventions

### Common Pitfalls

- Don't confuse warm migration (`plan.Spec.Warm`) with raw copy mode (`plan.Spec.SkipGuestConversion`)
- Remember that Critical conditions block execution completely
- VDDK is required for both warm migration AND raw copy mode in VMware
- Always check provider type before applying provider-specific logic

## Contributing

When making changes:

1. Create feature branches from latest upstream
2. Follow the existing code patterns
3. Add appropriate tests
4. Ensure validation logic is comprehensive
5. Update documentation as needed

This project values migration reliability and user experience - always consider the impact of changes on the migration workflow.
