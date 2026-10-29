# Provider Settings Reference

| Metadata | Value |
|----------|-------|
| **Last Updated** | January 22, 2026 |
| **Applies To** | Forklift v2.11 |
| **Maintainer** | Forklift Team |

This document details the provider-specific settings available in `spec.settings` of the Provider CR.

## Overview

Provider settings are optional configuration parameters that customize provider behavior. They are specified as key-value pairs in the Provider CR:

```yaml
apiVersion: forklift.konveyor.io/v1beta1
kind: Provider
metadata:
  name: my-provider
spec:
  type: vsphere
  url: https://vcenter.example.com/sdk          # API endpoint
  secret:
    name: vsphere-credentials
    namespace: openshift-mtv
  settings:
    vddkInitImage: "my-registry/vddk:v8.0"      # Container image FQIN
    sdkEndpoint: vcenter
```

---

## VMware vSphere

vSphere has the most extensive settings options for optimizing disk transfer.

| Setting | Values | Default | Description |
|---------|--------|---------|-------------|
| `vddkInitImage` | Container image FQIN | None | VDDK container image for disk transfers. Required for production migrations. Overrides global `VDDK_IMAGE`. |
| `sdkEndpoint` | `vcenter`, `esxi` | `vcenter` | SDK endpoint type. Use `esxi` for direct ESXi host connections. |
| `useVddkAioOptimization` | `"true"`, `"false"` | `"false"` | Enable VDDK AIO (Async I/O) optimization for improved performance. |
| `vddkConfig` | JSON string | None | Advanced VDDK configuration options. |
| `esxiCloneMethod` | `vib`, `ssh`, `api` | `ssh` | Method for direct ESXi disk cloning. `vib` uses VIB package, `ssh` uses SSH transfer, `api` uses the vSphere `CopyVirtualDisk` API (see below). |

#### `api` clone method

The `api` method performs copy-offload clones with the vSphere
`VirtualDiskManager.CopyVirtualDisk` API, so it needs neither the VIB installed
on the host nor SSH access to it.

The call must be issued against the **ESXi host's own SDK endpoint**. The same
request sent to vCenter copies the data correctly but does not offload to VAAI
XCOPY, which would be a silent, large performance regression. The populator
therefore logs in to the host directly, and every ESXi host that may perform a
clone needs a `Host` CR:

```yaml
apiVersion: forklift.konveyor.io/v1beta1
kind: Host
metadata:
  name: esxi-host-36
  namespace: openshift-mtv
spec:
  id: host-36                 # inventory host ID
  provider:
    name: my-vsphere-provider
    namespace: openshift-mtv
  ipAddress: 10.0.0.10        # must serve the host SDK on 443
  secret:
    name: esxi-host-36        # keys: user, password
    namespace: openshift-mtv
```

`spec.ipAddress` is used as the SDK endpoint, with no fallback to the host's
management IP, so copy traffic stays on the network the `Host` CR designates.

If a plan uses copy offload with `esxiCloneMethod: api` and any host that could
run a clone has no Ready `Host` CR with credentials, the plan reports a Critical
`HostCredentialsNotReady` condition and does not start. When the storage map
pins `dedicatedMigrationHosts`, every pinned host needs a `Host` CR.

This method applies only to VMDK-backed disks; RDM and vVol disks continue to
use their existing paths.

**Offload reporting:** the `vib` and `ssh` methods report whether XCOPY was used
in the populator's `xcopyUsed` status; the `api` method cannot, because ESXi does
not expose VAAI clone counters through the vSphere API. `xcopyUsed` is therefore
always `0` with this method, and says nothing about whether offload happened. If
a migration is slower than expected, check the storage array's own dashboard for
offload activity.

### VDDK Container Image

The VDDK (Virtual Disk Development Kit) container image is required for vSphere migrations. You must build and provide your own VDDK image due to VMware licensing.

The `vddkInitImage` setting accepts a fully qualified container image name (FQIN):

```yaml
settings:
  vddkInitImage: "my-registry.example.com/vddk:v8.0"
```

**Image resolution order:**
1. Provider-level `vddkInitImage` setting (if specified)
2. Global `VDDK_IMAGE` from ForkliftController (fallback)

See the [VMware VDDK documentation](https://developer.vmware.com/web/sdk/7.0/vddk) for building the container image.

### SDK Endpoint

Use `sdkEndpoint: esxi` when connecting directly to an ESXi host API endpoint instead of vCenter:

```yaml
spec:
  type: vsphere
  url: https://esxi-host.example.com/sdk    # ESXi API endpoint
  settings:
    sdkEndpoint: esxi
```

### AIO Optimization

Enable asynchronous I/O for better disk transfer performance:

```yaml
settings:
  useVddkAioOptimization: "true"
```

**Note:** AIO optimization requires compatible VDDK versions and may not work with all storage configurations.

---

## Red Hat Virtualization (oVirt)

oVirt providers currently have no additional settings.

| Setting | Values | Default | Description |
|---------|--------|---------|-------------|
| (none) | - | - | No provider-specific settings |

---

## OpenStack

OpenStack providers currently have no additional settings beyond what's in the secret.

| Setting | Values | Default | Description |
|---------|--------|---------|-------------|
| (none) | - | - | No provider-specific settings |

---

## OpenShift Virtualization

OpenShift providers have no additional settings. The "host" provider (same cluster) requires no URL.

| Setting | Values | Default | Description |
|---------|--------|---------|-------------|
| (none) | - | - | No provider-specific settings |

---

## OVA

OVA providers have no additional settings. The provider URL specifies the NFS endpoint where OVA files are stored.

| Setting | Values | Default | Description |
|---------|--------|---------|-------------|
| (none) | - | - | No provider-specific settings |

**Provider URL Format:** `nfs://server/export/path`

The NFS endpoint is mounted by the OVA provider server deployment (see [ForkliftController Settings](./forkliftcontroller-settings.md#container-image-settings) for the container image configuration).

---

## Amazon EC2

EC2 providers require settings to specify the target availability zone.

| Setting | Required | Values | Default | Description |
|---------|----------|--------|---------|-------------|
| `target-az` | **Yes** | AZ name | None | Target availability zone for EBS volumes (e.g., `us-east-1a`) |
| `target-region` | No | Region name | Provider region | Target region (for cross-region migrations) |

### Target Availability Zone

The `target-az` setting is critical for EC2 migrations. EBS volumes are AZ-specific and must be created in an AZ where OpenShift worker nodes exist:

```yaml
spec:
  type: ec2
  secret:
    name: ec2-credentials
  settings:
    target-az: us-east-1a
```

**Why is this required?**
- EBS volumes can only attach to EC2 instances in the same AZ
- OpenShift nodes run as EC2 instances
- The CSI driver cannot attach volumes from different AZs

### Finding Valid AZs

Query your OpenShift nodes to find available AZs:

```bash
kubectl get nodes -o jsonpath='{.items[*].metadata.labels.topology\.kubernetes\.io/zone}' | tr ' ' '\n' | sort -u
```

---

## Hyper-V

| Setting | Required | Values | Default | Description |
|---------|----------|--------|---------|-------------|
| `winrmPort` | No | Port number | `5986` | WinRM HTTPS port on the Hyper-V host |

---

## Nutanix

| Setting | Required | Values | Default | Description |
|---------|----------|--------|---------|-------------|
| `prismType` | No | "central" or "element" | Probe endpoint | Prism Central or Prism Element, if not specified forklift probes the endpoint |
| `clusterUuid` | No | UUID | None | If Prism Central is managing multiple clusters, inventory targets only the given cluster |

---

## Summary Table

| Setting | vSphere | oVirt | OpenStack | OpenShift | OVA | EC2 | HyperV | Nutanix |
|---------|:-------:|:-----:|:---------:|:---------:|:---:|:---:|:------:|:-------:|
| `vddkInitImage` | Yes | - | - | - | - | - | - | - |
| `sdkEndpoint` | Yes | - | - | - | - | - | - | - |
| `useVddkAioOptimization` | Yes | - | - | - | - | - | - | - |
| `vddkConfig` | Yes | - | - | - | - | - | - | - |
| `esxiCloneMethod` | Yes | - | - | - | - | - | - | - |
| `target-az` | - | - | - | - | - | **Req** | - | - |
| `target-region` | - | - | - | - | - | Opt | - | - |
| `winrmPort` | - | - | - | - | - | - | Opt | - |
| `prismType` | - | - | - | - | - | - | - | Opt |
| `clusterUuid` | - | - | - | - | - | - | - | Opt |

**Legend:** Yes = Supported, Opt = Optional, **Req** = Required, - = Not applicable
