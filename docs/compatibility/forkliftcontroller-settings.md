# ForkliftController Settings Reference

| Metadata | Value |
|----------|-------|
| **Last Updated** | January 22, 2026 |
| **Applies To** | Forklift v2.11 |
| **Maintainer** | Forklift Team |

This document details the configuration options available in the ForkliftController CR and the environment variables that control Forklift behavior.

## ForkliftController CR

The ForkliftController CR is managed by the Forklift operator and controls the deployment configuration.

```yaml
apiVersion: forklift.konveyor.io/v1beta1
kind: ForkliftController
metadata:
  name: forklift-controller
  namespace: openshift-mtv
spec:
  # Feature toggles
  feature_ui_plugin: true
  feature_validation: true
  feature_volume_populator: true
  feature_cli_download: true

  # Controller settings
  controller_max_vm_inflight: 20
  controller_precopy_interval: 60
  controller_log_level: 3

  # Container resources
  virt_v2v_container_limits_cpu: "4000m"
  virt_v2v_container_limits_memory: "8Gi"
```

---

## Feature Gates

Feature gates enable or disable specific Forklift capabilities.

### Core Features

| Setting | Default | Description |
|---------|---------|-------------|
| `feature_ui_plugin` | `true` | Enable OpenShift Console UI plugin |
| `feature_validation` | `true` | Enable VM validation with OPA policies |
| `feature_volume_populator` | `true` | Enable volume populator for oVirt/OpenStack |
| `feature_cli_download` | `true` | Enable kubectl-mtv CLI download service |
| `feature_auth_required` | `true` | Require authentication for inventory API |

### Migration Features

| Setting | Default | Environment Variable | Description |
|---------|---------|---------------------|-------------|
| `controller_vsphere_incremental_backup` | `true` | `FEATURE_VSPHERE_INCREMENTAL_BACKUP` | Enable CBT-based incremental backup for vSphere warm migrations |
| `controller_ovirt_warm_migration` | `true` | `FEATURE_OVIRT_WARM_MIGRATION` | Enable warm migration from oVirt |
| `feature_copy_offload` | `false` | `FEATURE_COPY_OFFLOAD` | Enable storage copy offload (XCOPY) |
| `feature_ocp_live_migration` | `false` | `FEATURE_OCP_LIVE_MIGRATION` | Enable OpenShift cross-cluster live migration |
| `feature_vmware_system_serial_number` | `true` | `FEATURE_VMWARE_SYSTEM_SERIAL_NUMBER` | Use VMware system serial number for migrated VMs |
| `controller_static_udn_ip_addresses` | `false` | `FEATURE_STATIC_UDN_IP_ADDRESSES` | Enable static IP addresses with User Defined Networks |
| `controller_retain_precopy_importer_pods` | `false` | `FEATURE_RETAIN_PRECOPY_IMPORTER_PODS` | Retain importer pods during warm migration (debugging) |
| `feature_ova_appliance_management` | `false` | `FEATURE_OVF_APPLIANCE_MANAGEMENT` | Enable appliance management for OVF-based providers |

### Feature Requirements

| Feature | Minimum OpenShift Version | Notes |
|---------|--------------------------|-------|
| `feature_vmware_system_serial_number` | 4.20+ | Requires CNV support |
| UDN MAC support | 4.20+ | Automatic, no feature gate |
| InsecureSkipVerify for ImageIO | 4.21+ | Automatic, no feature gate |

---

## Migration Settings

### Concurrency and Limits

| Setting | Default | Environment Variable | Description |
|---------|---------|---------------------|-------------|
| `controller_max_vm_inflight` | `20` | `MAX_VM_INFLIGHT` | Maximum concurrent VM migrations |
| `controller_max_concurrent_reconciles` | `10` | `MAX_CONCURRENT_RECONCILES` | Maximum concurrent controller reconciles |

### Timing Settings

| Setting | Default | Unit | Environment Variable | Description |
|---------|---------|------|---------------------|-------------|
| `controller_precopy_interval` | `60` | minutes | `PRECOPY_INTERVAL` | Interval between warm migration precopies |
| `controller_snapshot_removal_timeout_minuts` | `120` | minutes | `SNAPSHOT_REMOVAL_TIMEOUT` | Timeout for snapshot removal |
| `controller_snapshot_status_check_rate_seconds` | `10` | seconds | `SNAPSHOT_STATUS_CHECK_RATE` | Rate for checking snapshot status |
| `controller_vddk_job_active_deadline_sec` | `300` | seconds | `VDDK_JOB_ACTIVE_DEADLINE` | Deadline for VDDK validation job |
| `controller_tls_connection_timeout_sec` | `5` | seconds | `TLS_CONNECTION_TIMEOUT` | TLS connection timeout |
| `controller_cdi_export_token_ttl` | `720` | minutes | `CDI_EXPORT_TOKEN_TTL` | CDI export token TTL |

### Retry Settings

| Setting | Default | Environment Variable | Description |
|---------|---------|---------------------|-------------|
| `controller_cleanup_retries` | `10` | `CLEANUP_RETRIES` | Maximum cleanup retry attempts |
| `controller_snapshot_removal_check_retries` | `20` | `SNAPSHOT_REMOVAL_CHECK_RETRIES` | Maximum snapshot removal check retries |
| `controller_max_parent_backing_retries` | `10` | `MAX_PARENT_BACKING_RETRIES` | Maximum retries for parent backing lookup |

### Storage Settings

| Setting | Default | Environment Variable | Description |
|---------|---------|---------------------|-------------|
| `controller_filesystem_overhead` | `10` | `FILESYSTEM_OVERHEAD` | Filesystem overhead percentage |
| `controller_block_overhead` | `0` | `BLOCK_OVERHEAD` | Block storage fixed overhead (bytes) |

---

## Container Resource Settings

### virt-v2v Container

| Setting | Default | Environment Variable | Description |
|---------|---------|---------------------|-------------|
| `virt_v2v_container_limits_cpu` | `4000m` | `VIRT_V2V_CONTAINER_LIMITS_CPU` | CPU limit |
| `virt_v2v_container_limits_memory` | `8Gi` | `VIRT_V2V_CONTAINER_LIMITS_MEMORY` | Memory limit |
| `virt_v2v_container_requests_cpu` | `1000m` | `VIRT_V2V_CONTAINER_REQUESTS_CPU` | CPU request |
| `virt_v2v_container_requests_memory` | `1Gi` | `VIRT_V2V_CONTAINER_REQUESTS_MEMORY` | Memory request |

### Hook Container

| Setting | Default | Environment Variable | Description |
|---------|---------|---------------------|-------------|
| `hooks_container_limits_cpu` | `1000m` | `HOOKS_CONTAINER_LIMITS_CPU` | CPU limit |
| `hooks_container_limits_memory` | `1Gi` | `HOOKS_CONTAINER_LIMITS_MEMORY` | Memory limit |
| `hooks_container_requests_cpu` | `100m` | `HOOKS_CONTAINER_REQUESTS_CPU` | CPU request |
| `hooks_container_requests_memory` | `150Mi` | `HOOKS_CONTAINER_REQUESTS_MEMORY` | Memory request |

### OVA Provider Server Container

| Setting | Default | Environment Variable | Description |
|---------|---------|---------------------|-------------|
| `ova_container_limits_cpu` | `1000m` | `OVA_CONTAINER_LIMITS_CPU` | CPU limit |
| `ova_container_limits_memory` | `1Gi` | `OVA_CONTAINER_LIMITS_MEMORY` | Memory limit |
| `ova_container_requests_cpu` | `100m` | `OVA_CONTAINER_REQUESTS_CPU` | CPU request |
| `ova_container_requests_memory` | `512Mi` | `OVA_CONTAINER_REQUESTS_MEMORY` | Memory request |

### HyperV Provider Server Container

| Setting | Default | Environment Variable | Description |
|---------|---------|---------------------|-------------|
| `hyperv_container_limits_cpu` | `1000m` | `HYPERV_CONTAINER_LIMITS_CPU` | CPU limit |
| `hyperv_container_limits_memory` | `1Gi` | `HYPERV_CONTAINER_LIMITS_MEMORY` | Memory limit |
| `hyperv_container_requests_cpu` | `100m` | `HYPERV_CONTAINER_REQUESTS_CPU` | CPU request |
| `hyperv_container_requests_memory` | `512Mi` | `HYPERV_CONTAINER_REQUESTS_MEMORY` | Memory request |

### Volume Populator Container

| Setting | Default | Environment Variable | Description |
|---------|---------|---------------------|-------------|
| `populator_container_limits_cpu` | `1000m` | `POPULATOR_CONTAINER_LIMITS_CPU` | CPU limit |
| `populator_container_limits_memory` | `1Gi` | `POPULATOR_CONTAINER_LIMITS_MEMORY` | Memory limit |
| `populator_container_requests_cpu` | `100m` | `POPULATOR_CONTAINER_REQUESTS_CPU` | CPU request |
| `populator_container_requests_memory` | `512Mi` | `POPULATOR_CONTAINER_REQUESTS_MEMORY` | Memory request |

---

## Container Image Settings

These settings define the container images used by Forklift components. Container images are specified as fully qualified image names (FQIN), e.g., `quay.io/kubev2v/forklift-virt-v2v:latest`.

| Setting | Environment Variable | Description |
|---------|---------------------|-------------|
| `virt_v2v_image_fqin` | `VIRT_V2V_IMAGE`, `RELATED_IMAGE_VIRT_V2V` | Container image for virt-v2v guest conversion pods |
| `vddk_image` | `VDDK_IMAGE` | Container image for VMware VDDK (can be overridden per-provider via `spec.settings.vddkInitImage`) |
| `ova_provider_server_fqin` | `OVA_PROVIDER_SERVER_IMAGE` | Container image for OVA provider server deployments (one per OVA provider) |
| `hyperv_provider_server_fqin` | `HYPERV_PROVIDER_SERVER_IMAGE` | Container image for HyperV provider server deployments (one per HyperV provider) |

### OVA/HyperV Provider Server Architecture

When you create an OVA or HyperV Provider, Forklift automatically creates a provider server deployment:

```
Provider CR (type: ova or hyperv)
  └── OVAProviderServer/HyperVProviderServer CR (auto-created)
        ├── PersistentVolume/PVC (mounts source storage)
        ├── Deployment (runs the global container image)
        └── Service (inventory API endpoint)
```

- The **global container image** (`ova_provider_server_fqin` / `hyperv_provider_server_fqin`) defines which image to run
- The **provider server CR** manages the per-provider deployment, storage mount, and service endpoint

---

## Advanced virt-v2v Settings

| Setting | Environment Variable | Description |
|---------|---------------------|-------------|
| `virt_v2v_dont_request_kvm` | `VIRT_V2V_DONT_REQUEST_KVM` | Don't request KVM device (use for nested virt) |
| `virt_v2v_extra_args` | `VIRT_V2V_EXTRA_ARGS` | Additional virt-v2v arguments |
| `virt_v2v_extra_conf_config_map` | `VIRT_V2V_EXTRA_CONF_CONFIG_MAP` | ConfigMap with extra virt-v2v configuration |

---

## ConfigMaps

### OS Mapping ConfigMaps

| ConfigMap | Environment Variable | Description |
|-----------|---------------------|-------------|
| `forklift-ovirt-osmap` | `OVIRT_OS_MAP` | oVirt guest OS to template mapping |
| `forklift-vsphere-osmap` | `VSPHERE_OS_MAP` | vSphere guest OS to template mapping |
| `forklift-virt-customize` | `VIRT_CUSTOMIZE_MAP` | virt-customize script mappings |

---

## Copy Offload Settings

For storage copy offload (XCOPY) feature:

| Setting | Default | Environment Variable | Description |
|---------|---------|---------------------|-------------|
| `controller_host_lease_namespace` | `openshift-mtv` | `HOST_LEASE_NAMESPACE` | Namespace for host lease objects |
| `controller_host_lease_duration_seconds` | `10` | `HOST_LEASE_DURATION_SECONDS` | Host lease duration |

---

## Logging and Debugging

| Setting | Default | Environment Variable | Description |
|---------|---------|---------------------|-------------|
| `controller_log_level` | `3` | `LOG_LEVEL` | Log verbosity (0-9, higher = more verbose) |

### Profiler Settings

| Setting | Environment Variable | Description |
|---------|---------------------|-------------|
| `controller_profile_kind` | `PROFILE_KIND` | Profiler type (cpu, heap, etc.) |
| `controller_profile_path` | `PROFILE_PATH` | Path for profile output |
| `controller_profile_duration` | `PROFILE_DURATION` | Profile duration |

---

## Component Enable/Disable

| Component | Setting | Default |
|-----------|---------|---------|
| UI Plugin | `feature_ui_plugin` | `true` |
| Validation | `feature_validation` | `true` |
| Volume Populator | `feature_volume_populator` | `true` |
| CLI Download | `feature_cli_download` | `true` |

---

## Example Configuration

```yaml
apiVersion: forklift.konveyor.io/v1beta1
kind: ForkliftController
metadata:
  name: forklift-controller
  namespace: openshift-mtv
spec:
  # Enable all features
  feature_ui_plugin: true
  feature_validation: true
  feature_volume_populator: true
  feature_cli_download: true

  # Migration features
  controller_vsphere_incremental_backup: true
  controller_ovirt_warm_migration: true
  feature_copy_offload: false
  feature_ocp_live_migration: false

  # Concurrency
  controller_max_vm_inflight: 30
  controller_max_concurrent_reconciles: 15

  # Timing
  controller_precopy_interval: 30
  controller_snapshot_removal_timeout_minuts: 180

  # virt-v2v resources (for large VMs)
  virt_v2v_container_limits_cpu: "8000m"
  virt_v2v_container_limits_memory: "16Gi"
  virt_v2v_container_requests_cpu: "2000m"
  virt_v2v_container_requests_memory: "4Gi"

  # Logging
  controller_log_level: 5
```
