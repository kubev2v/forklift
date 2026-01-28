# EC2 Provider: AWS Bill of Materials

| Metadata | Value |
|----------|-------|
| **Last Updated** | January 27, 2026 |
| **Applies To** | Forklift v2.11+ |
| **Maintainer** | Forklift Team |

This document helps users understand and estimate AWS costs associated with migrating EC2 instances to OpenShift Virtualization using Forklift. It details all AWS API calls made and resources created during migration.

---

## Cost Summary Overview

When migrating EC2 instances, AWS charges for:

1. **EBS Snapshot Storage** - Temporary storage during migration
2. **EBS Volume Storage** - Permanent storage for migrated disks
3. **API Calls** - Minimal cost, typically negligible
4. **Data Transfer** - Cross-AZ transfer if source and target AZ differ

---

## AWS API Calls

### Inventory Collection (Periodic)

These API calls run periodically to maintain the inventory cache. Default refresh interval is configurable.

| API Call | Frequency | Purpose |
|----------|-----------|---------|
| `DescribeInstances` | Every refresh | List all EC2 instances |
| `DescribeVolumes` | Every refresh | List all EBS volumes |
| `DescribeVpcs` | Every refresh | List VPCs for network mapping |
| `DescribeSubnets` | Every refresh | List subnets for network mapping |
| `DescribeSecurityGroups` | Every refresh | List security groups |

**Cost Impact:** EC2 Describe API calls are free. No charges apply for inventory collection.

---

### Per-VM Migration API Calls

These API calls are made during the migration of each VM.

#### Instance Operations

| API Call | Count per VM | Purpose |
|----------|--------------|---------|
| `DescribeInstances` | 3-10 | Check instance state, get block device mappings |
| `StopInstances` | 1 | Stop VM before snapshot (if running) |

#### Snapshot Operations (per disk)

| API Call | Count per Disk | Purpose |
|----------|----------------|---------|
| `CreateSnapshot` | 1 | Create EBS snapshot from source volume |
| `DescribeSnapshots` | 5-20 | Poll until snapshot completes |
| `ModifySnapshotAttribute` | 1* | Share snapshot with target account |
| `DeleteSnapshot` | 1 | Cleanup after migration |

*Only for cross-account migrations

#### Volume Operations (per disk)

| API Call | Count per Disk | Purpose |
|----------|----------------|---------|
| `DescribeVolumes` | 5-15 | Check volume state, get metadata |
| `CreateVolume` | 1 | Create volume from snapshot in target AZ |
| `DeleteVolume` | 0-1 | Only on migration failure (cleanup) |

#### STS Operations (cross-account only)

| API Call | Count per Migration | Purpose |
|----------|---------------------|---------|
| `GetCallerIdentity` | 1 | Get target account ID for sharing |

**Cost Impact:** AWS API calls are low cost. For typical migrations, cost should be around $0.01 per VM (using us-east Jan 2026 prices).

---

## AWS Resources Created

### EBS Snapshots (Temporary)

| Resource | Quantity | Lifecycle | Billing |
|----------|----------|-----------|---------|
| EBS Snapshot | 1 per disk | Created at start, deleted after migration | Per GB-month stored |

**Important:** Snapshots are automatically cleaned up after successful migration. Failed migrations also clean up snapshots. Snapshot storage is only billed for the duration they exist (typically minutes to hours).

#### Snapshot Cost Formula

```
Snapshot Cost = Σ (Disk Size in GB × Hours Stored / 730) × Snapshot Rate per GB-month
```

**Current Rates (as of 2026, varies by region):**
- Standard snapshots: ~$0.05 per GB-month
- For a 100GB disk stored for 1 hour: ~$0.007

### EBS Volumes (Permanent)

| Resource | Quantity | Lifecycle | Billing |
|----------|----------|-----------|---------|
| EBS Volume | 1 per disk | Created during migration, **persists after migration** | Per GB-month stored |

**Important:** EBS volumes remain after migration as they back the PersistentVolumeClaims (PVCs) used by the migrated VM. These are your ongoing storage costs.

#### Volume Cost Formula

```
Monthly Volume Cost = Σ (Disk Size in GB × Volume Rate per GB-month)
```

#### Ongoing Storage Costs

```
Monthly Volume Cost = Total Storage (GB) × Volume Rate
```

**Example Calculation:**
- 2,000 GB × $0.08 (gp3) = **$160.00 per month**

---

## Migration Timeline and Resource Lifecycle

```
┌─────────────────────────────────────────────────────────────────────────────┐
│ Migration Timeline for a Single VM with 2 Disks                             │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│ Phase 1: Pre-Transfer                                                       │
│ ├─ StopInstances (if running)                                               │
│ └─ Wait for instance to stop                                                │
│                                                                             │
│ Phase 2: Snapshot Creation                                                  │
│ ├─ CreateSnapshot (disk 1) ──────┐                                          │
│ ├─ CreateSnapshot (disk 2) ──────┼── Snapshots created (billing starts)     │
│ └─ Poll DescribeSnapshots ───────┘                                          │
│                                                                             │
│ Phase 3: Share Snapshots (cross-account only)                               │
│ ├─ GetCallerIdentity                                                        │
│ └─ ModifySnapshotAttribute × 2                                              │
│                                                                             │
│ Phase 4: Volume Creation                                                    │
│ ├─ CreateVolume (disk 1) ────────┐                                          │
│ ├─ CreateVolume (disk 2) ────────┼── Volumes created (billing starts)       │
│ └─ Poll DescribeVolumes ─────────┘                                          │
│                                                                             │
│ Phase 5: PV/PVC Creation                                                    │
│ └─ Create Kubernetes PVs/PVCs referencing EBS volumes                       │
│                                                                             │
│ Phase 6: VM Creation                                                        │
│ └─ Create KubeVirt VirtualMachine with PVC references                       │
│                                                                             │
│ Phase 7: Cleanup                                                            │
│ └─ DeleteSnapshot × 2 ───────── Snapshots deleted (billing stops)           │
│                                                                             │
│ Post-Migration:                                                             │
│ └─ Volumes remain (ongoing billing for PVC storage)                         │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Resource Tagging

Forklift tags all created AWS resources for identification and cost allocation:

| Tag Key | Value | Purpose |
|---------|-------|---------|
| `forklift.konveyor.io/vmID` | EC2 Instance ID | Link resource to source VM |
| `forklift.konveyor.io/vm-name` | VM Name | Human-readable identification |
| `forklift.konveyor.io/volume` | Source Volume ID | Track snapshot source |
| `forklift.konveyor.io/original-volume` | Original Volume ID | Track volume lineage |
| `forklift.konveyor.io/snapshot` | Snapshot ID | Link volume to snapshot |

**Tip:** Use these tags in AWS Cost Explorer to track migration-related costs.

---

## Cross-Account vs Same-Account Migration

### Same-Account Migration

- Snapshots and volumes created in the same AWS account
- No snapshot sharing required
- Simpler IAM permissions
- **Requirement:** OpenShift cluster must have access to the same AWS account

### Cross-Account Migration

- Source account: snapshots created here
- Target account: volumes created here (must match OpenShift cluster's account)
- Additional API calls for snapshot sharing
- **Requirement:** Both account credentials in provider secret

| Cost Factor | Same-Account | Cross-Account |
|-------------|--------------|---------------|
| API Calls | Lower | Slightly higher |
| Snapshot Storage | Source account | Source account |
| Volume Storage | Source account | Target account |
| Data Transfer | Within account | May cross accounts |

---

## Cost Optimization Tips

### 1. Batch Migrations During Off-Peak Hours
Snapshots complete faster during periods of lower AWS API load, reducing snapshot storage time.

### 2. Clean Up Source Resources
After successful migration validation, consider:
- Terminating source EC2 instances
- Deleting source EBS volumes (ensure migration is verified first!)

### 3. Right-Size Before Migration
Reduce disk sizes in AWS before migration when possible. Migration preserves volume sizes.

### 4. Use Appropriate Volume Types
Volumes are created with the same type as the source. Consider if all source volumes need their current performance tier.

### 5. Monitor with AWS Cost Explorer
Use the Forklift tags to track migration costs:
```
Tag: forklift.konveyor.io/vmID
```

---

## IAM Permissions for Migration

Minimum required IAM permissions for the source account:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "ec2:DescribeInstances",
        "ec2:DescribeVolumes",
        "ec2:DescribeSnapshots",
        "ec2:DescribeVpcs",
        "ec2:DescribeSubnets",
        "ec2:DescribeSecurityGroups",
        "ec2:StopInstances",
        "ec2:StartInstances",
        "ec2:CreateSnapshot",
        "ec2:DeleteSnapshot",
        "ec2:CreateTags"
      ],
      "Resource": "*"
    }
  ]
}
```

Additional permissions for cross-account migrations (source account):

```json
{
  "Effect": "Allow",
  "Action": [
    "ec2:ModifySnapshotAttribute"
  ],
  "Resource": "*"
}
```

Permissions for target account (cross-account only):

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "ec2:DescribeSnapshots",
        "ec2:DescribeVolumes",
        "ec2:CreateVolume",
        "ec2:DeleteVolume",
        "ec2:CreateTags",
        "sts:GetCallerIdentity"
      ],
      "Resource": "*"
    }
  ]
}
```

---

## Related Documentation

- [AWS EBS Pricing](https://aws.amazon.com/ebs/pricing/) - Current AWS pricing information
- [AWS EC2 Pricing](https://aws.amazon.com/ec2/pricing/) - EC2 API pricing (free tier)
