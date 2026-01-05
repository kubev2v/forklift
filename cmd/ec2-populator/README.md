# EC2 Volume Populator

Creates Kubernetes PersistentVolumes from AWS EBS snapshots for VM migration to OpenShift.

## Quick Overview

The EC2 populator enables VM migrations by:
- Taking snapshots of source EBS volumes in AWS
- Creating new EBS volumes from snapshots in the OpenShift cluster's AZ
- Handling AWS credentials securely with standard environment variables

## What This Populator Does

The EC2 populator provisions an EBS-backed PV:

1. **Verifies** the EBS snapshot exists in the region
2. **Creates** a new EBS volume from snapshot in target AZ (AWS-native restore)
3. **Waits** for the volume to become available
4. **Creates** a Kubernetes PV with EBS CSI driver referencing the new volume
5. **Pre-binds** the PV to the **prime PVC** (not the user's PVC)
6. **Exits** - populator-machinery handles rebinding

**Snapshot AZ flexibility**: Snapshots are region-wide and can create volumes in **any AZ** within their region.
The populator always creates volumes in `targetAvailabilityZone` (where OpenShift workers run).

## The Prime PVC Pattern

**Standard Kubernetes volume populator flow**:

1. User (or Forklift EC2 migrator) creates **PVC** with `dataSourceRef` pointing to an `Ec2VolumePopulator` CR
2. **Populator-machinery** creates **prime PVC** (name: `prime-{pvc-uid}`) in the **same namespace** and with the **same `storageClassName`** as the triggering PVC
3. **Populator pod** runs, creates EBS volume + **PV with ClaimRef → prime PVC**, using the **triggering PVC's namespace and storage class**
4. **Prime PVC binds** to the PV (EBS volume now accessible in cluster)
5. **Populator-machinery rebinds** PV from prime PVC to user's PVC
6. **Prime PVC deleted** after successful rebind
7. **User's PVC** now bound to EBS volume

At this point, the user's PVC is bound to the EBS volume. **But that's not the end**.

## Resource Cleanup

**EC2 Populator:**
- Deletes the **prime PVC** after it has rebound the PV from the prime PVC to the user PVC.

**Forklift EC2 migration controller:**
- **Source snapshots** – created for each VM disk and **deleted after the migration completes**.
- **Populator/source PVCs** – the EBS‑backed PVCs used as the **source side** for copying into cluster storage are **deleted after a successful copy**.
- **EBS volumes** – deleted automatically by the EBS CSI driver when those PVCs are deleted, according to the PV **reclaim policy** (default `Delete`).
- **Populator AWS credential Secrets** – short‑lived Secrets created for the populator pods and **deleted during the migration cleanup phase**.

## Design Rationale

### Why Snapshots?

**Point-in-time consistency and source isolation**.

1. **Immutable backup**: Source volumes deleted immediately after snapshot
2. **Standard AWS mechanism**: Well-tested, reliable AWS-native backup

### Why Create New Volume from Snapshot?

**AWS-native restore with CSI integration**.

1. **In-AWS transfer**: Snapshot → volume within AWS datacenters (fast, no internet)
2. **CSI driver support**: EBS CSI driver mounts volumes, not snapshots
3. **Fresh volume**: Independent of source, can have different type/size/AZ

## Installation

**Required before using the EC2 populator:**

1. **Install the EC2 populator CRD:**

   **If using the Forklift operator:** the operator **automatically installs** the Ec2VolumePopulator CRD for you as part of its deployment.
   
   **For standalone testing:** you can install the CRD manually:

   ```bash
   kubectl apply -f operator/config/crd/bases/forklift.konveyor.io_ec2volumepopulators.yaml
   ```

2. **Configure forklift populator-controller with EC2 populator image:**
   
   **If using Forklift operator:** EC2 populator image is automatically set during operator deployment (see `operator/roles/forkliftcontroller/defaults/main.yml`)
   
   **For standalone testing:** Set the environment variable manually (when not using forklift operator):
   ```bash
   kubectl set env deployment/forklift-volume-populator-controller \
     EC2_POPULATOR_IMAGE=quay.io/kubev2v/ec2-populator:latest \
     -n openshift-cnv
   ```
   
   **Note:** The forklift-volume-populator-controller watches for Ec2VolumePopulator CRs and creates populator pods using this image.

3. **Ensure EBS CSI driver is installed** in your cluster

## API Reference

### Ec2VolumePopulator Custom Resource

```yaml
apiVersion: forklift.konveyor.io/v1beta1
kind: Ec2VolumePopulator
metadata:
  name: my-vm-disk-0
  namespace: default
spec:
  # Required: AWS region (where snapshot exists and OpenShift runs)
  region: us-west-2
  
  # Required: Availability zone where OpenShift worker nodes are
  # Must be in same region, determines where EBS volume is created
  targetAvailabilityZone: us-west-2b
  
  # Required: EBS snapshot ID to populate from
  snapshotId: snap-0123456789abcdef0
  
  # Required: Kubernetes secret with AWS credentials
  # Must contain: AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY
  secretName: aws-credentials
```

### PersistentVolumeClaim with Populator

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: my-vm-disk
  namespace: default
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 50Gi
  storageClassName: gp3
  dataSourceRef:
    apiGroup: forklift.konveyor.io
    kind: Ec2VolumePopulator
    name: my-vm-disk-0
```

## Configuration

### AWS Credentials Secret

**Format**: Standard AWS environment variable names

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: aws-credentials
  namespace: default
type: Opaque
stringData:
  AWS_ACCESS_KEY_ID: AKI...AMPLE
  AWS_SECRET_ACCESS_KEY: wJalrXUtnFEM...EXAMPLEKEY
```

### Required IAM Permissions

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "ec2:DescribeSnapshots",
        "ec2:CreateVolume",
        "ec2:DescribeVolumes",
        "ec2:CreateTags"
      ],
      "Resource": "*"
    }
  ]
}
```
