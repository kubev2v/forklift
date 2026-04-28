# Forklift Metrics Reference

Catalog of the Prometheus metrics scraped from the forklift-controller, with
types, labels, descriptions, and example queries. For architecture, labeling
conventions, and developer guidance see [metrics.md](./metrics.md).

All metrics below are defined in
[pkg/monitoring/metrics/forklift-controller/metrics.go](../pkg/monitoring/metrics/forklift-controller/metrics.go),
recorded by
[migration_metrics.go](../pkg/monitoring/metrics/forklift-controller/migration_metrics.go)
and
[plan_metrics.go](../pkg/monitoring/metrics/forklift-controller/plan_metrics.go),
and served on the controller's `:8443/metrics` endpoint.

> **Note:** The example queries below use `oc metrics` (or equivalently
> `kubectl metrics`), a kubectl plugin for querying Prometheus / Thanos on
> OpenShift clusters. Install it from
> [kubectl-metrics](https://github.com/yaacov/kubectl-metrics#installation).

---

### Common labels

Most metrics carry a subset of these labels:

- **`status`** -- lifecycle state of the migration or plan. Terminal states
  used by counters: `Succeeded`, `Failed`, `Canceled`. Gauges may also use:
  `Executing`, `Running`, `Pending`, `Blocked`, `Deleted`.
- **`provider`** -- source virtualization platform the VMs are migrated from.
  One of: `openshift`, `vsphere`, `ovirt`, `openstack`, `ova`, `ec2`, `hyperv`.
- **`mode`** -- migration strategy. `Cold` shuts the source VM down and copies
  disks in one pass. `Warm` replicates incrementally while the source VM keeps
  running, then cuts over.
- **`target`** -- destination cluster. `Local` means the same OpenShift cluster
  that runs the controller (no URL configured). `Remote` means a different
  cluster reached via an explicit URL.

Additional labels that appear only on specific metrics are explained inline
below.

---

### `mtv_migrations_status_total`

| | |
|---|---|
| **Type** | Counter |
| **Labels** | `status`, `provider`, `mode`, `target` |
| **Description** | Running count of VM migrations that reached a terminal state. |

Incremented once per migration when it transitions to Succeeded, Failed, or
Canceled. Deduplication ensures each migration UID is counted at most once per
status.

```bash
# Total successful migrations from vSphere
oc metrics query --query 'mtv_migrations_status_total{status="Succeeded", provider="vsphere"}'

# Rate of failed migrations over the last hour
oc metrics query --query 'rate(mtv_migrations_status_total{status="Failed"}[1h])'

# Discover all mtv migration metrics available on the cluster
oc metrics discover --keyword mtv_migrations

# Inspect labels present on this metric
oc metrics labels --metric mtv_migrations_status_total
```

### `mtv_workload_migrations_status_total`

| | |
|---|---|
| **Type** | Counter |
| **Labels** | `status`, `provider`, `mode`, `target`, `plan` |
| **Description** | Same as `mtv_migrations_status_total` but adds a `plan` label for per-plan correlation. |

The extra `plan` label is the Kubernetes UID of the `Plan` resource. This
allows you to break down migration counts per plan when multiple plans share the
same provider/mode/target combination.

```bash
# Succeeded migrations for a specific plan
oc metrics query --query 'mtv_workload_migrations_status_total{status="Succeeded", plan="<plan-uid>"}'
```

### `mtv_plans_status`

| | |
|---|---|
| **Type** | Gauge |
| **Labels** | `status`, `provider`, `mode`, `target` |
| **Description** | Current count of plans in each status. Recalculated every 10 seconds; stale combinations are reset to zero. |

Valid `status` values: Succeeded, Failed, Executing, Running, Pending,
Canceled, Blocked, Deleted.

```bash
# Number of currently executing plans
oc metrics query --query 'mtv_plans_status{status="Executing"}'

# Group plan counts by status
oc metrics query --query 'mtv_plans_status' --group-by status
```

### `mtv_plan_alert_status`

| | |
|---|---|
| **Type** | Gauge |
| **Labels** | `status`, `provider`, `mode`, `target`, `plan`, `plan_name`, `phase` |
| **Description** | Set to `1` for plans that are Succeeded, Failed, or Executing; deleted when the plan leaves that state. Designed for alerting rules. |

Extra labels beyond the common set:

- **`plan`** -- Kubernetes UID of the `Plan` resource.
- **`plan_name`** -- human-readable `.metadata.name` of the plan, so alerting
  rules can display a friendly name without needing a UID lookup.
- **`phase`** -- pipeline phase the plan is in or where it failed. `Completed`
  for succeeded plans, `Executing` for in-progress plans, or a comma-separated
  list of error phases reported by each failed VM.

```bash
# Check for any failed plans
oc metrics query --query 'mtv_plan_alert_status{status="Failed"} == 1'

# Show alert status grouped by plan name
oc metrics query --query 'mtv_plan_alert_status' --group-by plan_name
```

### `mtv_migration_duration_seconds`

| | |
|---|---|
| **Type** | Gauge |
| **Labels** | `provider`, `mode`, `target`, `plan` |
| **Description** | Wall-clock duration in seconds of the last successful migration for each plan. |

The `plan` label is the Kubernetes UID of the `Plan` resource, letting you see
how long the most recent successful migration took for each individual plan.

Set once when a migration succeeds, calculated as
`migration.Status.Completed - migration.Status.Started`.

```bash
# Duration of the last successful migration per plan
oc metrics query --query 'mtv_migration_duration_seconds' --group-by plan
```

### `mtv_migrations_duration_seconds`

| | |
|---|---|
| **Type** | Histogram |
| **Labels** | `provider`, `mode`, `target` |
| **Buckets** | 1h, 2h, 5h, 10h, 24h, 48h (in seconds) |
| **Description** | Distribution of successful migration durations. |

```bash
# Median migration duration for vSphere cold migrations
oc metrics query --query 'histogram_quantile(0.5, rate(mtv_migrations_duration_seconds_bucket{provider="vsphere", mode="Cold"}[24h]))'

# Duration histogram over the last 24 hours (range query, 1h steps)
oc metrics query-range \
  --query 'histogram_quantile(0.5, rate(mtv_migrations_duration_seconds_bucket[1h]))' \
  --start "-24h" --step "1h"
```

### `mtv_migration_data_transferred_bytes`

| | |
|---|---|
| **Type** | Gauge |
| **Labels** | `provider`, `mode`, `target`, `plan` |
| **Description** | Total bytes transferred across all VM disks for a migration. Updated on successful migration completion and also during plan execution for in-progress plans. |

The `plan` label is the Kubernetes UID of the `Plan` resource, so you can see
transfer volumes per plan.

Data is summed from the `DiskTransfer` and `DiskTransferV2v` pipeline steps.
The `Progress.Completed` value (in MB) is converted to bytes.

```bash
# Data transferred per plan
oc metrics query --query 'mtv_migration_data_transferred_bytes' --group-by plan
```

