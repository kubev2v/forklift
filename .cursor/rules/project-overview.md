# Forklift Project Overview

## What is Forklift?

Forklift is a comprehensive toolkit for migrating virtual machines from various source platforms to KubeVirt on Kubernetes. It provides enterprise-grade migration capabilities with support for warm migrations, guest conversions, and multiple virtualization platforms.

### Primary Use Cases
- **VMware to KubeVirt Migration**: Migrate VMs from vSphere environments
- **oVirt/RHV to KubeVirt**: Move VMs from Red Hat Virtualization
- **OpenStack to KubeVirt**: Migrate from OpenStack environments
- **OVA Import**: Import OVA files as KubeVirt VMs
- **Cross-cluster Migration**: Move VMs between different Kubernetes clusters

## Project Goals

### Migration Quality
- **Zero data loss**: Ensure complete and accurate VM migration
- **Minimal downtime**: Support warm migrations with incremental sync
- **Guest compatibility**: Automatically convert VMs to run on KubeVirt
- **Validation first**: Identify and prevent migration issues before execution

### Enterprise Requirements
- **Scale**: Handle hundreds of VMs in a single migration plan
- **Reliability**: Robust error handling and recovery mechanisms
- **Observability**: Comprehensive logging, metrics, and status reporting
- **Security**: Secure credential handling and data transfer

### Developer Experience
- **Kubernetes-native**: Standard CR/controller patterns
- **Extensible**: Provider adapters for different source platforms
- **Observable**: Rich status conditions and progress reporting
- **Testable**: Comprehensive test coverage and validation

## Key Concepts

### Migration Types

#### Cold Migration
- VM powered off during entire migration
- Simplest and most reliable approach
- Longer downtime but highest success rate

#### Warm Migration
- Uses Change Block Tracking (CBT) for incremental sync
- Minimal downtime (seconds to minutes)
- Requires CBT enabled on source VMs
- Supported for VMware and oVirt

#### Raw Copy Mode
- Skips guest OS conversion (`SkipGuestConversion: true`)
- Assumes VMs already have appropriate drivers
- Requires VDDK for VMware sources
- Faster but needs preparation

### Provider Adapters

Each source platform has a dedicated adapter:

#### VMware vSphere Adapter
- **Path**: `pkg/controller/plan/adapter/vsphere/`
- **Requirements**: VDDK for warm migration and raw copy
- **Features**: CBT support, guest tools integration, snapshot management
- **Validation**: VM tools, hardware compatibility, network mapping

#### oVirt/RHV Adapter  
- **Path**: `pkg/controller/plan/adapter/ovirt/`
- **Features**: Agent-based operations, direct storage access
- **Validation**: Agent connectivity, storage domain access, network configuration

#### OpenStack Adapter
- **Path**: `pkg/controller/plan/adapter/openstack/`
- **Features**: API-based operations, image conversion
- **Validation**: API credentials, network topology, image formats

#### OVA Adapter
- **Path**: `pkg/controller/plan/adapter/ova/`
- **Features**: Single-file VM import
- **Validation**: File accessibility, format compatibility

## Architecture Components

### Controllers

#### Plan Controller
- **Location**: `pkg/controller/plan/`
- **Purpose**: Manages migration plans and validation
- **Key Files**:
  - `controller.go`: Main reconciliation logic
  - `validation.go`: Pre-migration validation
  - `migration.go`: Migration execution orchestration

#### Migration Controller
- **Location**: `pkg/controller/migration/`
- **Purpose**: Executes individual migration runs
- **Responsibilities**: VM migration, progress tracking, error handling

#### Provider Controllers
- **Locations**: `pkg/controller/provider/*/`
- **Purpose**: Manage source provider connections
- **Types**: VMware, oVirt, OpenStack, OVA

### Validation Engine

#### Controller Validation
- **Language**: Go
- **Location**: `pkg/controller/plan/validation.go`
- **Purpose**: Runtime validation using Kubernetes APIs
- **Examples**: Resource existence, provider connectivity, VDDK availability

#### OPA Policy Validation
- **Language**: Rego
- **Location**: `validation/policies/`
- **Purpose**: Declarative validation rules
- **Examples**: VM compatibility, filesystem support, hardware validation

### Migration Pipeline

#### Phase-based Execution
1. **Initialize**: Setup migration context and resources
2. **DiskAllocation**: Create target PVCs and storage
3. **DiskTransfer**: Copy VM disk data to target storage
4. **ImageConversion**: Convert guest OS for KubeVirt (virt-v2v)
5. **VMCreation**: Create KubeVirt VirtualMachine resource
6. **Cutover**: Final sync and VM startup

#### Progress Tracking
- **Granular phases**: Each step reports progress percentage
- **Status conditions**: Rich condition types for different scenarios
- **Error recovery**: Retry logic and failure handling
- **User feedback**: Clear status messages and next steps

## Data Flow

### Migration Planning
```
User → Plan CR → Plan Controller → Validation → Ready Status
```

### Migration Execution
```
Plan → Migration CR → Migration Controller → Provider Adapter → Target Resources
```

### Validation Flow
```
Plan → Controller Validation → OPA Policies → Condition Status → User Feedback
```

## Integration Points

### Kubernetes APIs
- **Custom Resources**: Plan, Migration, Provider, NetworkMap, StorageMap
- **Native Resources**: PVC, Secret, ConfigMap, Service
- **KubeVirt**: VirtualMachine, VirtualMachineInstance, DataVolume

### External Systems
- **Source Platforms**: VMware vCenter, oVirt Engine, OpenStack APIs
- **Storage**: Various StorageClasses and CSI drivers
- **Networking**: CNI plugins, NetworkAttachmentDefinitions
- **Image Processing**: virt-v2v, VDDK, qemu tools

### Observability
- **Metrics**: Prometheus metrics for migration progress and health
- **Logging**: Structured logging with contextual information
- **Events**: Kubernetes events for significant migration milestones
- **Status**: Rich status conditions on all custom resources

## Development Workflow

### Code Organization
- **Modular design**: Clear separation between controllers, adapters, and utilities
- **Interface-driven**: Provider adapters implement common interfaces
- **Library code**: Shared utilities in `pkg/lib/`
- **Generated code**: API types and client code generation

### Testing Strategy
- **Unit tests**: Controller logic and utility functions
- **Integration tests**: End-to-end migration scenarios
- **OPA testing**: Policy validation with test data
- **Mock adapters**: Testing without actual infrastructure

### Validation Strategy
- **Multi-layer validation**: Both Go and OPA validation
- **Early detection**: Validate before migration starts
- **Clear feedback**: Actionable error messages
- **Provider-specific**: Tailored validation per source platform

This overview provides the foundation for understanding how Forklift works and how to contribute effectively to the project.
