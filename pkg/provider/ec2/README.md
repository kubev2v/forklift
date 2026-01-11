# EC2 Provider

Migrate EC2 instances to OpenShift Virtualization using EBS snapshots and direct volume provisioning with optional guest OS conversion. Supports both same-account and cross-account migrations via snapshot sharing.

## Overview

Cold migration of EC2 instances via EBS snapshot integration. Leverages direct EBS volume creation and AWS EBS CSI driver for volume access. Guest OS conversion (virt-v2v) is performed to ensure the migrated VM has the necessary drivers for OpenShift Virtualization.

**Pipeline:** EC2 Instance (stopped) → EBS Snapshots → (Snapshot Sharing for cross-account) → EBS Volumes → PVs/PVCs (EBS) → Guest Conversion (virt-v2v) → KubeVirt VM

### Migration Flow

1. **Snapshot Creation**: Provider creates EBS snapshots of stopped EC2 instance volumes.
2. **Snapshot Sharing** (cross-account only): If the source and target are in different AWS accounts, the snapshots are shared with the target account.
3. **Volume Creation**: Provider creates new EBS volumes from those snapshots in the target availability zone (in the target account).
4. **PV/PVC Provisioning**: Provider creates PersistentVolumes (pointing to the EBS volumes) and PersistentVolumeClaims.
5. **Guest Conversion**: A conversion pod is created to run `virt-v2v` on the migrated volumes, installing necessary drivers and configuring the guest OS.
6. **VM Creation**: KubeVirt VM created with migrated disks attached directly via the pre-bound PVCs.

**Storage Note:** EBS volumes are used directly via the AWS EBS CSI driver. The volumes are attached to the KubeVirt VM as PersistentVolumeClaims.

## Setup with kubectl-mtv

### 1. Create the EC2 Provider

#### Same-Account Migration (Simplest)

Use the cluster's AWS credentials automatically:

```bash
kubectl mtv create provider my-ec2 --type ec2 \
  --ec2-region us-east-1 \
  --username "$EC2_KEY" \
  --password "$EC2_SECRET" \
  --auto-target-credentials
```

The `--auto-target-credentials` flag automatically:
- Fetches target AWS credentials from the cluster secret (`kube-system/aws-creds`)
- Detects target availability zone from worker node topology labels

#### Provider Flags Reference

| Flag | Required | Description |
|------|----------|-------------|
| `--type ec2` | **Yes** | Provider type |
| `--ec2-region` | **Yes** | AWS region where EC2 instances are located |
| `--username` | **Yes** | Source AWS access key ID |
| `--password` | **Yes** | Source AWS secret access key |
| `--target-access-key-id` | No | Target account access key (cross-account only) |
| `--target-secret-access-key` | No | Target account secret key (cross-account only) |
| `--target-region` | No | Target region (defaults to provider region) |
| `--target-az` | No | Target availability zone (defaults to target-region + 'a') |
| `--auto-target-credentials` | No | Auto-fetch target credentials and AZ from cluster |

### 2. Explore the Inventory

After provider creation, explore available resources:

```bash
# List EC2 instances
kubectl mtv get inventory ec2-instance my-ec2

# List with query filter
kubectl mtv get inventory ec2-instance my-ec2 -q "where powerState = 'Off'"

# List EBS volumes
kubectl mtv get inventory ec2-volume my-ec2

# List EBS volume types (for storage mapping)
kubectl mtv get inventory ec2-volume-type my-ec2

# List networks (VPCs and Subnets)
kubectl mtv get inventory ec2-network my-ec2
```


## Manual Setup (YAML)

If you prefer YAML manifests:

### Provider Secret

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: ec2-credentials
  namespace: openshift-mtv
type: Opaque
stringData:
  region: us-east-1
  accessKeyId: AKIA...
  secretAccessKey: ...
```

#### Secret Fields

| Field | Required | Description |
|-------|----------|-------------|
| `region` | **Yes** | AWS region where EC2 instances are located (e.g., `us-east-1`) |
| `accessKeyId` | **Yes** | AWS access key ID for authentication |
| `secretAccessKey` | **Yes** | AWS secret access key for authentication |
| `targetAccessKeyId` | No | Target account access key (cross-account migrations only) |
| `targetSecretAccessKey` | No | Target account secret key (cross-account migrations only) |

> **Note:** For same-account migrations, use the OpenShift cluster's AWS credentials to ensure the EBS CSI driver can access the migrated volumes.

### Provider Resource

```yaml
apiVersion: forklift.konveyor.io/v1beta1
kind: Provider
metadata:
  name: my-ec2
  namespace: openshift-mtv
spec:
  type: ec2
  secret:
    name: ec2-credentials
    namespace: openshift-mtv
  settings:
    target-az: us-east-1a
```

#### Provider Settings

| Setting | Required | Description |
|---------|----------|-------------|
| `target-az` | **Yes** | Target availability zone where EBS volumes will be created. Must match an AZ where OpenShift worker nodes run. |

**Why target-az?** EBS volumes are AZ-specific. If created in the wrong AZ, the CSI driver cannot attach them to worker nodes, causing migration to fail.

## Availability Zone Node Selection

By default, EC2 migrations automatically add a node selector to migrated VMs to ensure they run on nodes in the same availability zone as their EBS volumes. This is required because EBS volumes can only be attached to EC2 instances (and thus OpenShift nodes) in the same AZ.

### How It Works

1. The provider's `spec.settings.target-az` defines where EBS volumes are created
2. During migration, a node selector `topology.kubernetes.io/zone=<target-az>` is automatically added to the target VM
3. Kubernetes schedules the VM on a node in that AZ, ensuring volume attachment succeeds

### Disabling Zone Node Selection

If you need to disable this behavior (e.g., if you manage node selection differently), set `skipZoneNodeSelector: true` in the Plan:

```bash
# Using kubectl-mtv patch (if supported) or via YAML:
kubectl patch plan migrate-from-ec2 --type=merge -p '{"spec":{"skipZoneNodeSelector":true}}'
```

| `skipZoneNodeSelector` | Behavior |
|------------------------|----------|
| `false` (default) | Adds `topology.kubernetes.io/zone=<target-az>` node selector to VMs |
| `true` | No zone-based node selector added |

## Requirements

**Source Environment:**
- EC2 instances with EBS volumes only (instance store not supported)
- IAM permissions: `ec2:Describe*`, `ec2:CreateSnapshot`, `ec2:DeleteSnapshot`, `ec2:StopInstances`, `ec2:CreateVolume`, `ec2:DeleteVolume`
- For cross-account migrations, additional IAM permission: `ec2:ModifySnapshotAttribute`

**Target Environment:**
- ROSA
- OSD AWS
