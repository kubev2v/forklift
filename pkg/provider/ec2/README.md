# EC2 Provider

Migrate EC2 instances to OpenShift Virtualization using EBS snapshots and volume populators.

## Overview

Cold migration of EC2 instances via EBS snapshot integration. Leverages the EC2 Volume Populator for disk provisioning and AWS EBS CSI driver for volume access.

**Pipeline:** EC2 Instance (stopped) → EBS Snapshots → Volume Populator → PVs/PVCs (EBS) → KubeVirt VM

**Note:** EC2 migration currently does NOT perform guest OS conversion. VMs are migrated as-is and must already have KubeVirt-compatible drivers (virtio) for proper operation.

### Migration Flow

1. **Snapshot Creation**: Provider creates EBS snapshots of stopped EC2 instance volumes
2. **Volume Population**: EC2 Volume Populator creates EBS volumes from snapshots and provisions Kubernetes PVs
3. **VM Creation**: KubeVirt VM created with migrated disks attached directly

**Storage Note:** EBS volumes are used directly via the AWS EBS CSI driver. The volumes are attached to the KubeVirt VM as PersistentVolumeClaims.

## Setup

### 1. Provider Secret

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: ec2-credentials
  namespace: konveyor-forklift
type: Opaque
stringData:
  region: us-east-1
  accessKeyId: AKIA...
  secretAccessKey: ...
```

### 2. Provider

**Required:** Configure the target availability zone where EBS volumes will be created. This must match the AZ where your OpenShift worker nodes run.

```yaml
apiVersion: forklift.konveyor.io/v1beta1
kind: Provider
metadata:
  name: my-ec2
  namespace: konveyor-forklift
spec:
  type: ec2
  secret:
    name: ec2-credentials
  settings:
    target-az: us-east-1a        # REQUIRED: Must match OpenShift worker nodes AZ
```

**Why target-az Required?** EBS volumes are AZ-specific. If created in the wrong AZ, the CSI driver cannot attach them to worker nodes, causing migration to fail.

**Note:** The AWS region is configured in the provider secret (see step 1), not in provider settings.

### 3. Storage Mapping (Required)

**Critical:** Storage mapping tells the populator which StorageClass to use for EBS volumes. The source name must match the EBS volume type.

```yaml
apiVersion: forklift.konveyor.io/v1beta1
kind: StorageMap
metadata:
  name: ec2-storage
  namespace: konveyor-forklift
spec:
  provider:
    source:
      name: my-ec2
    destination:
      name: host
  map:
    - source:
        name: gp3  # EBS volume type from source
      destination:
        storageClass: gp3-csi  # StorageClass with EBS CSI driver
    - source:
        name: gp2
      destination:
        storageClass: gp3-csi
    - source:
        name: io1
      destination:
        storageClass: gp3-csi
```

**Important:** The destination StorageClass must use the AWS EBS CSI driver (`provisioner: ebs.csi.aws.com`) and be accessible from the target namespace.

### 4. Network Mapping

Map EC2 subnets to OpenShift networks. The source name must match the subnet ID (not VPC ID).

```yaml
apiVersion: forklift.konveyor.io/v1beta1
kind: NetworkMap
metadata:
  name: ec2-network
  namespace: konveyor-forklift
spec:
  provider:
    source:
      name: my-ec2
    destination:
      name: host
  map:
    - source:
        name: subnet-5678abcdef  # Subnet ID (e.g., subnet-0123456789abcdef0)
      destination:
        type: pod  # or multus network
```

### 5. Migration Plan

```yaml
apiVersion: forklift.konveyor.io/v1beta1
kind: Plan
metadata:
  name: migrate-from-ec2
spec:
  provider:
    source:
      name: my-ec2
    destination:
      name: host
  map:
    network:
      name: ec2-network
    storage:
      name: ec2-storage
  targetNamespace: migrated-vms
  vms:
    - id: i-1234567890abcdef0
```

## Requirements

**Source Environment:**
- EC2 instances with EBS volumes only (instance store not supported)
- IAM permissions: `ec2:Describe*`, `ec2:CreateSnapshot`, `ec2:DeleteSnapshot`, `ec2:StopInstances`, `ec2:CreateVolume`, `ec2:DeleteVolume`

**Target Environment:**
- ROSA
- OSD AWS

## Architecture

### Provider Components

- **inventory/** - Collects EC2 instances, volumes, networks, security groups via AWS SDK v2
  - **collector/** - Resource collectors for VM, network, storage inventory
  - **web/** - REST API handlers exposing inventory data
- **controller/** - Orchestrates migration pipeline with snapshot lifecycle
  - **adapter/** - Provider adapter implementing Client interface for EC2 operations
  - **builder/** - Creates Ec2VolumePopulator CRs, PVCs with dataSourceRef, VirtualMachine specs
  - **validator/** - Validates VM is stopped, uses EBS only, mappings exist, snapshot limits
  - **migrator/** - Manages migration phases: snapshots → populate → create VM

### External Components

- **EC2 Volume Populator** (`cmd/ec2-populator/`) - Creates EBS volumes from snapshots and provisions PVs
