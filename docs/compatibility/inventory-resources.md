# Inventory Resources Reference

| Metadata | Value |
|----------|-------|
| **Last Updated** | January 22, 2026 |
| **Applies To** | Forklift v2.11 |
| **Maintainer** | Forklift Team |

This document details the inventory resource types available from each provider through the Forklift inventory API.

## Overview

Forklift discovers and exposes source infrastructure resources through its inventory API. These resources are used for:
- VM selection in migration plans
- Network and storage mapping configuration
- Capacity planning and validation

## Resource Types by Provider

### VMware vSphere

| Resource Type | API Endpoint | Description |
|---------------|--------------|-------------|
| **VM** | `/providers/vsphere/{uid}/vms` | Virtual machines |
| **Network** | `/providers/vsphere/{uid}/networks` | Port groups and distributed port groups |
| **Datastore** | `/providers/vsphere/{uid}/datastores` | Storage datastores |
| **Host** | `/providers/vsphere/{uid}/hosts` | ESXi hosts |
| **Cluster** | `/providers/vsphere/{uid}/clusters` | vSphere clusters |
| **Datacenter** | `/providers/vsphere/{uid}/datacenters` | vSphere datacenters |
| **Folder** | `/providers/vsphere/{uid}/folders` | VM and infrastructure folders |
| **Resource Pool** | `/providers/vsphere/{uid}/resourcepools` | Resource pools |

#### vSphere VM Properties

| Property | Type | Description |
|----------|------|-------------|
| `id` | string | VM managed object reference |
| `name` | string | VM display name |
| `uuid` | string | VM instance UUID |
| `firmware` | string | BIOS or EFI |
| `powerState` | string | poweredOn, poweredOff, suspended |
| `cpuCount` | int | Number of vCPUs |
| `memoryMB` | int | Memory in MB |
| `guestId` | string | Guest OS identifier |
| `disks` | []Disk | Attached virtual disks |
| `networks` | []Network | Connected networks |
| `host` | Reference | Current ESXi host |
| `concerns` | []Concern | Migration validation concerns |

---

### Red Hat Virtualization (oVirt)

| Resource Type | API Endpoint | Description |
|---------------|--------------|-------------|
| **VM** | `/providers/ovirt/{uid}/vms` | Virtual machines |
| **Network** | `/providers/ovirt/{uid}/networks` | Logical networks |
| **NIC Profile** | `/providers/ovirt/{uid}/nicprofiles` | vNIC profiles |
| **Storage Domain** | `/providers/ovirt/{uid}/storagedomains` | Storage domains |
| **Disk** | `/providers/ovirt/{uid}/disks` | Virtual disks |
| **Disk Profile** | `/providers/ovirt/{uid}/diskprofiles` | Disk profiles |
| **Host** | `/providers/ovirt/{uid}/hosts` | Hypervisor hosts |
| **Cluster** | `/providers/ovirt/{uid}/clusters` | oVirt clusters |
| **Datacenter** | `/providers/ovirt/{uid}/datacenters` | oVirt datacenters |

#### oVirt VM Properties

| Property | Type | Description |
|----------|------|-------------|
| `id` | string | VM UUID |
| `name` | string | VM name |
| `status` | string | up, down, suspended, etc. |
| `cluster` | Reference | Parent cluster |
| `host` | Reference | Current host (if running) |
| `cpuCores` | int | CPU cores |
| `cpuSockets` | int | CPU sockets |
| `memory` | int | Memory in bytes |
| `disks` | []DiskAttachment | Attached disks |
| `nics` | []NIC | Network interfaces |
| `concerns` | []Concern | Migration validation concerns |

---

### OpenStack

| Resource Type | API Endpoint | Description |
|---------------|--------------|-------------|
| **VM (Server)** | `/providers/openstack/{uid}/vms` | Nova instances |
| **Network** | `/providers/openstack/{uid}/networks` | Neutron networks |
| **Subnet** | `/providers/openstack/{uid}/subnets` | Network subnets |
| **Volume** | `/providers/openstack/{uid}/volumes` | Cinder volumes |
| **Volume Type** | `/providers/openstack/{uid}/volumetypes` | Volume types |
| **Snapshot** | `/providers/openstack/{uid}/snapshots` | Volume snapshots |
| **Flavor** | `/providers/openstack/{uid}/flavors` | Instance flavors |
| **Image** | `/providers/openstack/{uid}/images` | Glance images |
| **Project** | `/providers/openstack/{uid}/projects` | Keystone projects |
| **Region** | `/providers/openstack/{uid}/regions` | OpenStack regions |

#### OpenStack VM Properties

| Property | Type | Description |
|----------|------|-------------|
| `id` | string | Instance UUID |
| `name` | string | Instance name |
| `status` | string | ACTIVE, SHUTOFF, etc. |
| `tenantID` | string | Project/tenant ID |
| `flavorID` | string | Flavor reference |
| `imageID` | string | Boot image reference |
| `addresses` | map | Network addresses |
| `volumes` | []VolumeAttachment | Attached volumes |
| `concerns` | []Concern | Migration validation concerns |

---

### OpenShift Virtualization

| Resource Type | API Endpoint | Description |
|---------------|--------------|-------------|
| **VM** | `/providers/openshift/{uid}/vms` | KubeVirt VirtualMachines |
| **Network** | `/providers/openshift/{uid}/networks` | NetworkAttachmentDefinitions |
| **Storage Class** | `/providers/openshift/{uid}/storageclasses` | Kubernetes StorageClasses |
| **Namespace** | `/providers/openshift/{uid}/namespaces` | Kubernetes namespaces |
| **PVC** | `/providers/openshift/{uid}/pvcs` | PersistentVolumeClaims |
| **DataVolume** | `/providers/openshift/{uid}/datavolumes` | CDI DataVolumes |

#### OpenShift VM Properties

| Property | Type | Description |
|----------|------|-------------|
| `id` | string | VM UID |
| `name` | string | VM name |
| `namespace` | string | Kubernetes namespace |
| `status` | string | Running, Stopped, etc. |
| `conditions` | []Condition | VM conditions |
| `dataVolumes` | []DataVolumeRef | Associated DataVolumes |
| `networks` | []Network | Network interfaces |
| `concerns` | []Concern | Migration validation concerns |

---

### OVA

| Resource Type | API Endpoint | Description |
|---------------|--------------|-------------|
| **VM** | `/providers/ova/{uid}/vms` | VMs from OVA/OVF files |
| **Network** | `/providers/ova/{uid}/networks` | Networks defined in OVF |
| **Disk** | `/providers/ova/{uid}/disks` | Virtual disks in OVA |

#### OVA VM Properties

| Property | Type | Description |
|----------|------|-------------|
| `id` | string | VM identifier |
| `name` | string | VM name from OVF |
| `ovaPath` | string | Path to OVA file |
| `cpuCount` | int | Number of CPUs |
| `memoryMB` | int | Memory in MB |
| `disks` | []Disk | Virtual disks |
| `networks` | []Network | Network definitions |
| `concerns` | []Concern | Migration validation concerns |

---

### Amazon EC2

| Resource Type | API Endpoint | Description |
|---------------|--------------|-------------|
| **Instance (VM)** | `/providers/ec2/{uid}/vms` | EC2 instances |
| **Network** | `/providers/ec2/{uid}/networks` | VPCs and subnets |
| **Volume** | `/providers/ec2/{uid}/volumes` | EBS volumes |
| **Volume Type** | `/providers/ec2/{uid}/volumetypes` | EBS volume types (gp3, io2, etc.) |

#### EC2 Instance Properties

| Property | Type | Description |
|----------|------|-------------|
| `id` | string | Instance ID (i-xxx) |
| `name` | string | Name tag value |
| `instanceType` | string | EC2 instance type |
| `state` | string | running, stopped, etc. |
| `availabilityZone` | string | AZ location |
| `vpcId` | string | VPC ID |
| `subnetId` | string | Subnet ID |
| `volumes` | []Volume | Attached EBS volumes |
| `networkInterfaces` | []ENI | Network interfaces |
| `tags` | map | AWS resource tags |
| `concerns` | []Concern | Migration validation concerns |

#### EC2 Tag Filtering

EC2 supports filtering VMs by AWS tags:

```
/providers/ec2/{uid}/vms?label.environment=production
```

---

### Hyper-V

| Resource Type | API Endpoint | Description |
|---------------|--------------|-------------|
| **VM** | `/providers/hyperv/{uid}/vms` | Hyper-V virtual machines |
| **Network** | `/providers/hyperv/{uid}/networks` | Virtual switches |
| **Disk** | `/providers/hyperv/{uid}/disks` | Virtual hard disks |

#### Hyper-V VM Properties

| Property | Type | Description |
|----------|------|-------------|
| `id` | string | VM identifier |
| `name` | string | VM name |
| `state` | string | Running, Off, etc. |
| `cpuCount` | int | Number of processors |
| `memoryMB` | int | Memory in MB |
| `generation` | int | VM generation (1 or 2) |
| `disks` | []Disk | Virtual hard disks |
| `networks` | []Network | Network adapters |
| `concerns` | []Concern | Migration validation concerns |

---

## Resource Summary Matrix

| Resource | vSphere | oVirt | OpenStack | OpenShift | OVA | EC2 | HyperV |
|----------|:-------:|:-----:|:---------:|:---------:|:---:|:---:|:------:|
| VMs | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| Networks | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| Storage | Datastores | Storage Domains | Volumes | StorageClasses | Disks | EBS Volumes | Disks |
| Hosts | Yes | Yes | - | - | - | - | - |
| Clusters | Yes | Yes | - | - | - | - | - |
| Datacenters | Yes | Yes | - | - | - | - | - |
| Folders | Yes | - | - | - | - | - | - |
| Resource Pools | Yes | - | - | - | - | - | - |
| Flavors | - | - | Yes | - | - | - | - |
| Images | - | - | Yes | - | - | - | - |
| Projects | - | - | Yes | - | - | - | - |
| Subnets | - | - | Yes | - | - | Yes | - |
| Namespaces | - | - | - | Yes | - | - | - |
| PVCs | - | - | - | Yes | - | - | - |
| DataVolumes | - | - | - | Yes | - | - | - |
| NIC Profiles | - | Yes | - | - | - | - | - |
| Disk Profiles | - | Yes | - | - | - | - | - |
| Volume Types | - | - | Yes | - | - | Yes | - |
| Snapshots | - | - | Yes | - | - | - | - |
| Regions | - | - | Yes | - | - | - | - |

---

## Concerns and Validation

All VM resources include a `concerns` field containing validation results:

| Concern Level | Description |
|---------------|-------------|
| `Critical` | Migration will fail - must be resolved |
| `Warning` | Migration may have issues - review recommended |
| `Advisory` | Informational - no action required |

Common concern categories:
- Unsupported guest OS
- Incompatible hardware configuration
- Missing drivers
- Snapshot presence
- Shared storage conflicts
- Network configuration issues

---

## Querying Resources

### Basic Query

```bash
# List all VMs from a provider
curl -H "Authorization: Bearer $TOKEN" \
  https://forklift-inventory/providers/vsphere/{uid}/vms
```

### Filtering

```bash
# Filter by power state
curl https://forklift-inventory/providers/vsphere/{uid}/vms?powerState=poweredOff

# EC2: Filter by tag
curl https://forklift-inventory/providers/ec2/{uid}/vms?label.environment=prod
```

### Detail Levels (vSphere/oVirt)

```bash
# Get detailed VM information
curl https://forklift-inventory/providers/vsphere/{uid}/vms/{vmId}?detail=1
```
