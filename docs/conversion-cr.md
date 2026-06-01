# Conversion CR

The `Conversion` custom resource drives a single unit of VM workload — inspection, in-place conversion, or remote copy+convert. It decouples the pod lifecycle from Plan reconciliation and can be used standalone.

---

## Architecture

```
                        ┌──────────────────────────────────────────────────┐
                        │              Conversion Controller               │
                        │                                                  │
                        │  reconcile() ──► validate()                      │
                        │               ──► ConversionPipeline.Run()       │
                        │                       │                          │
                        │               ┌───────▼──────────┐               │
                        │               │  PhasePending    │               │
                        │               │  (snapshot owner │               │
                        │               │   resolution)    │               │
                        │               └───────┬──────────┘               │
                        │                       │                          │
                        │               ┌───────▼──────────┐               │
                        │               │  PhaseRunning    │               │
                        │               │  stage machine   │               │
                        │               └───────┬──────────┘               │
                        │                       │                          │
                        │          ┌────────────┼────────────┐             │
                        │          ▼            ▼            ▼             │
                        │    virt-v2v      DeepInspection  virt-v2v        │
                        │    pipeline      pipeline        pipeline        │
                        │  (Inspection/   (snapshot +     (InPlace/        │
                        │   Remote/        pod + fetch)    Remote)         │
                        │   InPlace)                                       │
                        └──────────────────────────────────────────────────┘

  Plan controller                 Standalone user
       │                                │
       ▼                                ▼
  creates Conversion CR ──────► Conversion CR (same schema)
  (labels + secrets pre-built)   (user supplies all fields)
```

---

## Deep inspection pod communication

The conversion controller communicates with the deep-inspection pod over a plain HTTP server the pod exposes on port **8080**. No service or ingress is involved — the controller connects directly to `pod.status.podIP`.

### Endpoints

| Endpoint   | Method | Description                                                                                                                       |
| ---------- | ------ | --------------------------------------------------------------------------------------------------------------------------------- |
| `/ready`   | GET    | Returns `200 OK` when detection is complete and results are ready to serve. Returns non-200 otherwise.                            |
| `/results` | GET    | Returns `200 OK` + JSON body with the full inspection result. Returns `503 Service Unavailable` while detection is still running. |

### Communication flow

```
Conversion controller                Deep-inspection pod
──────────────────────               ────────────────────
                                     [starts vm-migration-detective]
                                     [detection running...]
                                     [detection complete → HTTP server
                                      serving /ready and /results]

StagePodRunning:
  poll GET /ready ─────────────────► 200 OK
  ◄─────────────────────────────────
  advance to StageFetchingResults

StageFetchingResults:
  GET /results ────────────────────► 200 OK + JSON
  ◄───────────────────────────────── {
                                       "all_checks_passed": false,
                                       "all_concerns": [...],
                                       "os_info": {...},
                                       "filesystems": [...],
                                       "mountpoints": [...]
                                     }
  store → status.inspectionResult
  advance to StageRemoveSnapshot
```

### Timing and retry behaviour

- During `StagePodRunning` the controller polls `GET /ready` on every reconcile. While the pod has no IP yet, or `/ready` returns non-200, the reconciler requeues and waits.
- Once `/ready` returns `200`, the pipeline immediately advances to `StageFetchingResults` without waiting for the pod to exit — results are fetched while the pod is still alive.
- If the pod exits before `/ready` was seen (e.g. it crashed), the pipeline still advances past both stages. `StageFetchingResults` skips gracefully if the pod is no longer `Running`.
- `GET /results` returning `503` is treated as "not ready yet" and retried on the next reconcile. Any other non-200 is a hard error.
- Both HTTP calls have a **5-second timeout**. Connection errors are treated as transient and retried.

### Example response JSON schema (subset persisted on the CR)

```json
{
  "all_checks_passed": false,
  "all_concerns": [
    {
      "id": "fstab-by-path",
      "category": "Critical",
      "label": "Non-migrateable fstab entries",
      "message": "Fstab contains /dev/disk/by-path/ entries..."
    }
  ],
  "os_info": {
    "name": "Red Hat Enterprise Linux",
    "distro": "rhel",
    "major_version": "8",
    "architecture": "x86_64"
  },
  "filesystems": [
    { "device": "/dev/sda1", "type": "xfs", "uuid": "abc-123" }
  ],
  "mountpoints": [
    { "device": "/dev/sda1", "mount_point": "/" }
  ]
}
```

The controller maps this to `status.inspectionResult` on the Conversion CR, converting snake_case JSON keys to the Go struct field names. Only the fields shown above are persisted; the full pod output is available in the pod logs.

---

## Lifecycle

### Phase / Stage

`status.phase` is the high-level lifecycle state. `status.stage` is the fine-grained position within `Running`.

```
Pending ──► Running ──► Succeeded
               │
               ├──► Failed
               └──► Canceled
```

### Stage sequences

**virt-v2v pipeline** (`Inspection`, `InPlace`, `Remote`):

```
CreatingPod → PodRunning → Finished
```

**DeepInspection pipeline** (snapshot owned by controller):

```
CreatingSnapshot → WaitingForSnapshot → CreatingPod → PodRunning
  → FetchingResults → RemovingSnapshot → WaitingForSnapshotRemoval → Finished
```

**DeepInspection pipeline** (pre-supplied `SNAPSHOT_MOREF`):

```
CreatingPod → PodRunning → FetchingResults → Finished
```

Snapshot stages are skipped when `spec.settings.SNAPSHOT_MOREF` is set.

---

## Usage paths

### 1. Plan path (virt-v2v types)

The Plan controller creates `Inspection`, `InPlace`, and `Remote` Conversion CRs automatically during migration. The CR carries all pod configuration resolved from the Plan spec; no manual intervention is needed.

### 2. Plan + DeepInspection path (warm migration preflight)

When `UseConversionCR` is enabled and the plan is a warm vSphere migration, the Plan controller drives the `PreflightInspection` step through a **two phase lookup** that can reuse a compatible standalone CR before falling back to a plan owned one.

#### Compatibility check

Before a standalone CR can be reused, it must have been built with settings that produce equivalent inspection results. Two fields are checked:

| Field                   | Why it matters                                                                                        |
| ----------------------- | ----------------------------------------------------------------------------------------------------- |
| `spec.diskEncryption.type` | Must match what the plan would configure (`LUKS`, `Clevis`, or none). A mismatch means the pod cannot read the disk and the results are from an incompatible access path. |
| `spec.xfsCompatibility` | Selects a different container image (`DeepInspectionImageXFS` vs. the standard image). Results from a non-XFS image cannot substitute for an XFS-compat run, or vice versa. |

CRs that fail either check are silently skipped, they will not be deleted or modified.

#### Phase 1 - plan-owned CR (plan UID label set)

The controller first checks for a CR that already has `plan=<planUID>`. This CR is present when the plan previously started a DI run for this VM (either directly created in Phase 3, or promoted from a failed standalone in Phase 2).

| CR state            | Action                                                                                        |
| ------------------- | --------------------------------------------------------------------------------------------- |
| `Succeeded`         | Propagate `InspectionResult.Concerns` to the migration step. Advance on no critical concerns. |
| `Running`/`Pending` | Wait, requeue.                                                                                |
| `Failed`/`Canceled` | Fail the migration step immediately. No further retries.                                      |
| None found          | Fall through to Phase 2.                                                                      |

#### Phase 2 - standalone CR (no plan label, compatible settings)

If no plan owned CR exists, the controller searches for `DeepInspection` CRs that have **no** `plan` label and whose `diskEncryption.type` and `xfsCompatibility` match the plan. If more than one exists, the most actionable is chosen: Succeeded > Running/Pending > Failed.

| CR state            | Action                                                                                        |
| ------------------- | --------------------------------------------------------------------------------------------- |
| `Succeeded`         | Propagate `InspectionResult.Concerns` to the migration step. Advance on no critical concerns. |
| `Running`/`Pending` | Wait; requeue.                                                                                |
| `Failed`/`Canceled` | Delete the standalone CR, then fall through to Phase 3 to create the plan owned CR.          |
| None found          | Fall through to Phase 3.                                                                      |

The standalone CR is deleted before Phase 3 creates the replacement because `ensureConversion` performs a plan label-agnostic lookup: without deletion it would update the `Failed` CR in place, leaving the conversion controller with no reason to re-run its pipeline.

#### Phase 3 - create plan-owned CR

No existing CR of either kind was found. The controller creates the first plan owned CR (copies credentials, optionally the LUKS passphrase secret, sets `SNAPSHOT_MOREF`) and requeues. Phase 1 handles the CR from the next reconcile onward.

#### Full decision flow

```
PhasePreflightInspection
│
├─ Phase 1: find plan owned CR (plan=<UID>, compatible settings)
│   ├─ Succeeded  ──► propagate concerns - advance (or fail on critical)
│   ├─ Running    ──► wait
│   ├─ Failed     ──► fail migration step (no retry)
│   └─ none found ──► Phase 2
│
├─ Phase 2: find standalone CR (no plan label, compatible settings)
│   ├─ Succeeded  ──► propagate concerns - advance (or fail on critical)
│   ├─ Running    ──► wait
│   ├─ Failed     ──► delete standalone CR ──► Phase 3
│   └─ none found ──► Phase 3
│
└─ Phase 3: create plan owned CR
    └─ wait (Phase 1 handles Succeeded / Running / Failed next reconcile)
```

### 3. Standalone path

Any `Conversion` CR created directly (without a Plan) is reconciled identically. The user is responsible for providing all required fields.

A standalone `DeepInspection` CR created before a plan runs can be **reused by the plan** provided it has no `plan` label and its `spec.diskEncryption.type` and `spec.xfsCompatibility` match what the plan would create. See Phase 2 above.

---

## Spec reference

### Required fields (all types)

| Field                    | Description                                              |
| ------------------------ | -------------------------------------------------------- |
| `spec.type`              | `DeepInspection`, `Inspection`, `InPlace`, or `Remote`   |
| `spec.vm.id`             | vSphere managed object ID of the source VM               |
| `spec.connection.secret` | Reference to the credentials secret (see per-type notes) |

### Optional fields

| Field                                         | Description                                                                                                                                                                                                                 |
| --------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `spec.targetNamespace`                        | Namespace where the pod is created. Defaults to CR namespace.                                                                                                                                                               |
| `spec.image`                                  | Overrides the controller-default virt-v2v / deep-inspection image.                                                                                                                                                          |
| `spec.vddkImage`                              | VDDK init-container image. Required for `DeepInspection`; optional for others.                                                                                                                                              |
| `spec.disks`                                  | PVC-backed disks for `InPlace`/`Remote`. Populated by Plan; set manually for standalone.                                                                                                                                    |
| `spec.diskEncryption`                         | LUKS passphrase secret or Clevis config for encrypted disks.                                                                                                                                                                |
| `spec.settings`                               | Freeform key/value pairs injected as env vars into the pod. `SNAPSHOT_MOREF` has special meaning for `DeepInspection`. For `Inspection`, `V2V_remoteInspection=true` and `V2V_remoteInspectDisk_N` (0-indexed) must be set. |
| `spec.xfsCompatibility`                       | Selects the XFS-compatible virt-v2v image variant.                                                                                                                                                                          |
| `spec.localMigration`                         | Sets `LOCAL_MIGRATION=true` in the pod.                                                                                                                                                                                     |
| `spec.destination`                            | Remote destination cluster provider. Omit for local (host) cluster.                                                                                                                                                         |
| `spec.podSettings.serviceAccount`             | Pod service account.                                                                                                                                                                                                        |
| `spec.podSettings.nodeSelector`               | Node selector for the pod.                                                                                                                                                                                                  |
| `spec.podSettings.affinity`                   | Pod affinity/anti-affinity rules.                                                                                                                                                                                           |
| `spec.podSettings.transferNetworkAnnotations` | Network annotations copied to the pod.                                                                                                                                                                                      |

---

## Resource placement

Every resource associated with a Conversion CR has a specific namespace and cluster target. The rules exist because the conversion pod can only mount secrets and PVCs that reside in the same namespace and on the same cluster.

### Conversion CR

| Resource      | Namespace                                                | Cluster            |
| ------------- | -------------------------------------------------------- | ------------------ |
| Conversion CR | `Plan.Namespace` (plan path) or user-chosen (standalone) | management cluster |

The Conversion CR lives on the **management cluster** alongside the Plan. The controller that reconciles it also runs there.

### Conversion pod

| Resource       | Namespace                                         | Cluster                                                                    |
| -------------- | ------------------------------------------------- | -------------------------------------------------------------------------- |
| Conversion pod | `spec.targetNamespace` (defaults to CR namespace) | destination cluster (`spec.destination`, or management cluster if omitted) |

The pod is scheduled on the **destination cluster** in `spec.targetNamespace`. When `spec.destination` is empty the management cluster is used as destination.

### Connection secret

| Resource          | Namespace                                       | Cluster             |
| ----------------- | ----------------------------------------------- | ------------------- |
| Connection secret | same as conversion pod (`spec.targetNamespace`) | destination cluster |

The connection secret is mounted at `/etc/secret` inside the pod. Because a pod can only mount secrets from its **own namespace on its own cluster**, the secret must live in `spec.targetNamespace` on the destination cluster.

In the plan path the controller copies the source-provider credentials (including `url` and `fingerprint`) from the management cluster into `TargetNamespace` on the destination cluster before creating the CR. In the standalone path the user must create the secret in that same location.

### LUKS passphrase secret

| Resource             | Namespace                               | Cluster             |
| -------------------- | --------------------------------------- | ------------------- |
| LUKS secret (source) | `vm.LUKS.Namespace` or `Plan.Namespace` | management cluster  |
| LUKS secret (copy)   | `spec.targetNamespace`                  | destination cluster |

The original LUKS secret lives on the management cluster in the plan namespace. Because the conversion pod cannot reach across clusters or namespaces to mount a secret, the plan controller **copies** the secret into `spec.targetNamespace` on the destination cluster. `spec.diskEncryption.secret` always references this copy; the copy is mounted at `/etc/luks` inside the pod.

### PVCs (InPlace / Remote)

| Resource | Namespace              | Cluster             |
| -------- | ---------------------- | ------------------- |
| PVCs     | `spec.targetNamespace` | destination cluster |

Disks are represented as PVCs on the destination cluster. `spec.disks` entries reference them by name and namespace; the controller mounts them as block devices or filesystem volumes into the conversion pod.

### Summary

```
Management cluster                    Destination cluster
──────────────────                    ───────────────────
Conversion CR                         Conversion pod  (spec.targetNamespace)
Plan CR                               Connection secret (spec.targetNamespace)
Source LUKS secret ──copy──►          LUKS secret copy  (spec.targetNamespace)
                                      PVCs              (spec.targetNamespace)
```

---

## Connection secret keys

The secret referenced by `spec.connection.secret` is mounted at `/etc/secret` inside the pod and also injected as `V2V_`-prefixed environment variables.

| Key                  | Required by                             | Description                         |
| -------------------- | --------------------------------------- | ----------------------------------- |
| `user`               | all                                     | Source username                     |
| `password`           | all                                     | Source password                     |
| `url`                | `DeepInspection`                        | vSphere SDK endpoint URL            |
| `fingerprint`        | `DeepInspection` (insecure skip-verify) | TLS fingerprint of the vSphere host |
| `insecureSkipVerify` | optional                                | Skip TLS certificate verification   |

For virt-v2v types the Plan controller supplies the secret; for standalone `DeepInspection` the user must populate all keys.

---

## Standalone spec examples

### DeepInspection — conversion controller-owned snapshot

The controller creates the snapshot, runs the inspection pod, then removes the snapshot.

```yaml
apiVersion: forklift.konveyor.io/v1beta1
kind: Conversion
metadata:
  name: inspect-my-vm
  namespace: konveyor-forklift
spec:
  type: DeepInspection
  vm:
    id: vm-1234
  vddkImage: quay.io/example/vddk:latest
  connection:
    secret:
      namespace: konveyor-forklift
      name: vsphere-credentials
```

`vsphere-credentials` must contain `url`, `user`, `password`, and `fingerprint` (or `insecureSkipVerify: "true"`).

---

### DeepInspection — pre-supplied snapshot

Use when the snapshot already exists (e.g. created by the warm migration itinerary).

```yaml
apiVersion: forklift.konveyor.io/v1beta1
kind: Conversion
metadata:
  name: inspect-my-vm-warm
  namespace: konveyor-forklift
spec:
  type: DeepInspection
  vm:
    id: vm-1234
  vddkImage: quay.io/example/vddk:latest
  connection:
    secret:
      namespace: konveyor-forklift
      name: vsphere-credentials
  settings:
    SNAPSHOT_MOREF: snapshot-567
```

Snapshot stages are skipped. The controller does not own the snapshot and will not remove it.

---

### DeepInspection — LUKS-encrypted disks

```yaml
apiVersion: forklift.konveyor.io/v1beta1
kind: Conversion
metadata:
  name: inspect-luks-vm
  namespace: konveyor-forklift
spec:
  type: DeepInspection
  vm:
    id: vm-1234
  vddkImage: quay.io/example/vddk:latest
  connection:
    secret:
      namespace: konveyor-forklift
      name: vsphere-credentials
  diskEncryption:
    type: LUKS
    secret:
      namespace: konveyor-forklift
      name: luks-secret
```

The LUKS secret is mounted at `/etc/luks` inside the pod.

---

### DeepInspection — Clevis (NBDE)

No passphrase secret is needed; the pod uses tang/TPM2 network-bound unlock.

```yaml
apiVersion: forklift.konveyor.io/v1beta1
kind: Conversion
metadata:
  name: inspect-clevis-vm
  namespace: konveyor-forklift
spec:
  type: DeepInspection
  vm:
    id: vm-1234
  vddkImage: quay.io/example/vddk:latest
  connection:
    secret:
      namespace: konveyor-forklift
      name: vsphere-credentials
  diskEncryption:
    type: Clevis
```

---

### Inspection (virt-v2v inspector)

Kept for backward compatibility. Runs virt-v2v in inspection mode.

Two things must be provided in `spec.settings` for standalone use (the plan controller sets both automatically):

- `V2V_remoteInspection: "true"` — switches virt-v2v into remote inspection mode.
- `V2V_remoteInspectDisk_N` (0-indexed) — one entry per disk, value is the disk path from the source inventory (the parent backing file path for warm migrations).

```yaml
apiVersion: forklift.konveyor.io/v1beta1
kind: Conversion
metadata:
  name: inspect-vm-v2v
  namespace: konveyor-forklift
spec:
  type: Inspection
  vm:
    id: vm-1234
  connection:
    secret:
      namespace: konveyor-forklift
      name: v2v-credentials
  settings:
    V2V_remoteInspection: "true"
    V2V_remoteInspectDisk_0: "[datastore] vm/vm-disk1-flat.vmdk"
    V2V_remoteInspectDisk_1: "[datastore] vm/vm-disk2-flat.vmdk"
```

---

### InPlace

Converts disks in-place; no copy to destination cluster.

```yaml
apiVersion: forklift.konveyor.io/v1beta1
kind: Conversion
metadata:
  name: convert-inplace
  namespace: konveyor-forklift
spec:
  type: InPlace
  vm:
    id: vm-1234
  connection:
    secret:
      namespace: konveyor-forklift
      name: v2v-credentials
  disks:
    - name: my-pvc-0
      namespace: konveyor-forklift
      volumeMode: Block
      devicePath: /dev/block0
```

---

### Remote

Copies disks from the source provider and converts them on the destination cluster.

```yaml
apiVersion: forklift.konveyor.io/v1beta1
kind: Conversion
metadata:
  name: convert-remote
  namespace: konveyor-forklift
spec:
  type: Remote
  vm:
    id: vm-1234
  connection:
    secret:
      namespace: konveyor-forklift
      name: v2v-credentials
  disks:
    - name: my-pvc-0
      namespace: konveyor-forklift
      volumeMode: Block
      devicePath: /dev/block0
```

---

## Status fields

| Field                     | Description                                                                    |
| ------------------------- | ------------------------------------------------------------------------------ |
| `status.phase`            | `Pending`, `Running`, `Succeeded`, `Failed`, `Canceled`                        |
| `status.stage`            | Current pipeline stage (see stage sequences above)                             |
| `status.pod`              | Reference to the managed conversion pod                                        |
| `status.startTime`        | When `Running` was entered                                                     |
| `status.completionTime`   | When `Succeeded` or `Failed` was reached                                       |
| `status.snapshot`         | vSphere snapshot tracking (MoRef, task IDs, ownership flag)                    |
| `status.inspectionResult` | Deep-inspection outcome: `passed`, OS info, concerns, filesystems, mountpoints |
| `status.conditions`       | Standard Kubernetes conditions set by validation and the pipeline              |

### Inspection concerns

When `status.inspectionResult.concerns` contains entries with `category: Critical` or `category: Error`, the plan controller fails the `PreflightInspection` step and surfaces each concern message alongside the main error reason. The migration cannot proceed until the underlying issue is resolved.

**Example — critical concern surfaced in the migration UI:**

Critical concern example

The concern `Fstab contains /dev/disk/by-path/ entries which are not migrateable` was reported by the deep-inspection pod with category `Critical`. The plan controller propagated it as an additional reason under `VM deep inspection found critical concerns`, blocking the migration at the `PreflightInspection` step.

The raw concern is available on the Conversion CR:

```yaml
status:
  inspectionResult:
    allChecksPassed: false
    concerns:
      - id: fstab-by-path
        category: Critical
        label: Non-migrateable fstab entries
        message: "Fstab contains /dev/disk/by-path/ entries which are not migrateable. Found device: /dev/disk/by-path/pci-0000:03:00.0-scsi-0:0:1:0-part1 mounted at: /home/..."
```
