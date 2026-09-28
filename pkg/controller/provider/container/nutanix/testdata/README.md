# Nutanix Provider Test Data

This directory contains mock Nutanix Prism v3 API responses for development and testing.

## Overview

These JSON files simulate responses from a Nutanix Prism Central environment with:
- 2 clusters (production and development)
- 3 AHV hosts
- 6 VMs with various configurations
- 4 networks/subnets
- 3 storage containers
- 4 images (OS templates and ISOs)

## File Descriptions

### clusters_list.json
Response from `POST /api/nutanix/v3/clusters/list`

**Contains**: 2 Nutanix clusters
- `prod-cluster-01` - Production cluster with 2 nodes, 25 VMs
- `dev-cluster-01` - Development cluster with 1 node, 8 VMs

**Key fields**:
- `metadata.uuid` - Cluster UUID
- `metadata.name` - Cluster name
- `status.resources.config.build.version` - AOS version (6.8.2)
- `status.resources.nodes.hypervisor_server_list` - AHV hosts

### hosts_list.json
Response from `POST /api/nutanix/v3/hosts/list`

**Contains**: 3 AHV hypervisor nodes

| Host | Cluster | CPU Cores | Memory | VMs |
|------|---------|-----------|--------|-----|
| ahv-node-01 | prod-cluster-01 | 32 cores (2×16) | 256 GB | 15 |
| ahv-node-02 | prod-cluster-01 | 32 cores (2×16) | 256 GB | 10 |
| ahv-dev-node-01 | dev-cluster-01 | 16 cores (2×8) | 128 GB | 8 |

**Key fields**:
- `metadata.uuid` - Host UUID
- `spec.cluster_reference` - Parent cluster
- `status.resources.num_cpu_sockets/cores` - CPU topology
- `status.resources.memory_capacity_mib` - Memory in MiB
- `status.resources.hypervisor.hypervisor_full_name` - free-text hypervisor
  version string (e.g. "Nutanix 20240802.100"); hosts have no dedicated
  type-enum field like VMs do

### vms_list.json
Response from `POST /api/nutanix/v3/vms/list`

**Contains**: 6 VMs demonstrating various migration scenarios

| VM Name | OS | State | vCPUs | Memory | Disks | NICs | Boot Mode | Notes |
|---------|----|----|-------|--------|-------|------|-----------|-------|
| web-server-rhel8 | RHEL 8 | ON | 4 (2×2) | 8 GB | 1 | 1 | UEFI | Single disk, DHCP IP |
| db-server-rhel9 | RHEL 9 | ON | 8 (4×2) | 16 GB | 2 | 1 | UEFI | Multi-disk (OS + data) |
| win2022-app-server | Windows 2022 | ON | 8 (2×4) | 32 GB | 2 | 2 | UEFI | Multi-NIC, Q35 machine |
| ubuntu-test-vm | Ubuntu 22.04 | ON | 2 (1×2) | 4 GB | 1 | 1 | LEGACY | BIOS boot mode |
| powered-off-vm | Generic | OFF | 1 (1×1) | 2 GB | 1 | 1 | UEFI | Powered off state |
| secure-boot-vm | Generic | ON | 4 (2×2) | 8 GB | 1 | 1 | SECURE_BOOT | UEFI Secure Boot |

**Migration scenarios covered**:
- ✅ UEFI boot (standard)
- ✅ Legacy BIOS boot
- ✅ UEFI Secure Boot
- ✅ Multiple disks
- ✅ Multiple NICs
- ✅ Different machine types (PC, Q35)
- ✅ Powered ON and OFF states
- ✅ Nutanix Guest Tools installed
- ✅ Categories (tags/labels)

**Key fields**:
- `metadata.uuid` - VM UUID
- `spec.resources.power_state` - "ON" or "OFF"
- `spec.resources.num_sockets/num_vcpus_per_socket` - CPU topology
- `spec.resources.memory_size_mib` - Memory in MiB
- `spec.resources.boot_config.boot_type` - LEGACY, UEFI, or SECURE_BOOT
- `spec.resources.nic_list` - Network interfaces
- `spec.resources.disk_list` - Virtual disks
- `spec.resources.guest_tools` - Nutanix Guest Tools info
- `status.resources.host_reference` - Current host

### vm_detail_example.json
Response from `GET /api/nutanix/v3/vms/{uuid}`

**Contains**: Full detailed response for a single VM (web-server-rhel8)

This includes all fields that would be present when fetching a specific VM, including:
- Complete metadata with categories
- Full spec including all resources
- Detailed status with execution context
- Guest OS information
- Serial port configuration
- Protection policy state

Use this as reference for the complete VM object structure.

### subnets_list.json
Response from `POST /api/nutanix/v3/subnets/list`

**Contains**: 4 network subnets

| Subnet Name | VLAN | Network | Gateway | DHCP Pool |
|-------------|------|---------|---------|-----------|
| Production-VLAN-100 | 100 | 192.168.100.0/24 | 192.168.100.1 | .100-.200 |
| Production-VLAN-200 | 200 | 192.168.200.0/24 | 192.168.200.1 | .50-.150 |
| Dev-Network | 10 | 10.0.10.0/24 | 10.0.10.1 | .100-.200 |
| Management-Network | 1 | 10.10.1.0/24 | 10.10.1.1 | N/A |

**Key fields**:
- `metadata.uuid` - Subnet UUID
- `spec.resources.vlan_id` - VLAN tag
- `spec.resources.subnet_type` - "VLAN" or "OVERLAY"
- `spec.resources.ip_config` - IP configuration (CIDR, gateway, DHCP)

### storage_containers_list.json
Response from `POST /api/nutanix/v3/storage_containers/list`

**Contains**: 3 storage containers

| Container Name | Cluster | Capacity | Used | RF | Compression | Dedup |
|----------------|---------|----------|------|----|-----------:|------:|
| default-container-prod | prod-cluster-01 | 8 TB | 3 TB | 2 | ✅ | ❌ |
| ssd-container-prod | prod-cluster-01 | 2 TB | 0.5 TB | 2 | ✅ | ✅ |
| default-container-dev | dev-cluster-01 | 4 TB | 1 TB | 2 | ✅ | ❌ |

**Key fields**:
- `metadata.uuid` - Container UUID
- `status.resources.replication_factor` - Replication factor (RF)
- `status.resources.max_capacity_bytes` - Total capacity
- `status.resources.usage_stats` - Usage statistics
- `status.resources.compression_enabled` - Compression
- `status.resources.dedup_enabled` - Deduplication

### images_list.json
Response from `POST /api/nutanix/v3/images/list`

**Contains**: 4 images (OS templates and ISOs)

| Image Name | Type | OS | Size |
|------------|------|----|----- |
| RHEL-8.9-x86_64 | DISK_IMAGE | RHEL 8.9 | 2 GB |
| Ubuntu-22.04-LTS | DISK_IMAGE | Ubuntu 22.04 | 1.8 GB |
| Windows-Server-2022 | DISK_IMAGE | Windows 2022 | 8 GB |
| VirtIO-Drivers-Latest | ISO_IMAGE | Drivers | 512 MB |

**Key fields**:
- `metadata.uuid` - Image UUID
- `status.resources.image_type` - "DISK_IMAGE" or "ISO_IMAGE"
- `status.resources.size_bytes` - Image size
- `status.resources.architecture` - "X86_64"

### images_v4_list.json
Response from `GET /api/vmm/v4.0/content/images`

**Contains**: 2 images, in Prism Central's v4 Image Service shape

The v3 "image" kind (`images_list.json`, above) is what Prism Element
serves directly. On Prism Central, the v3 kind isn't reliably populated,
so images are instead listed via this v4 endpoint and reshaped by
`imageEntityFromV4()` into the same v3-style structure before
`applyImage()` runs, so both Prism modes share one mapping function.

**Key fields** (note the flatter, non-nested shape vs. the v3 files above):
- `extId` - Image UUID
- `type` - "DISK_IMAGE" or "ISO_IMAGE"
- `sizeBytes` - Image size
- `clusterLocationExtIds` - Clusters the image is registered on (a list;
  images aren't scoped to a single cluster in either Prism mode)

v4 has no equivalent of v3's `architecture` field, so it's always empty
when mapped from this source.

## Data Relationships

### Cluster → Hosts
- `prod-cluster-01` has hosts: `ahv-node-01`, `ahv-node-02`
- `dev-cluster-01` has host: `ahv-dev-node-01`

### Cluster → Networks
- `prod-cluster-01` has: Production-VLAN-100, Production-VLAN-200, Management-Network
- `dev-cluster-01` has: Dev-Network

### Cluster → Storage
- `prod-cluster-01` has: default-container-prod, ssd-container-prod
- `dev-cluster-01` has: default-container-dev

### VM → Host Placement
- `web-server-rhel8` → `ahv-node-01`
- `db-server-rhel9` → `ahv-node-02`
- `win2022-app-server` → `ahv-node-01`
- `ubuntu-test-vm` → `ahv-dev-node-01`
- `secure-boot-vm` → `ahv-node-02`

### VM → Network Connections
- Most VMs use single NIC
- `win2022-app-server` uses dual NICs (VLAN-100 and VLAN-200)

### VM → Storage
- Most VMs use default-container
- `db-server-rhel9` data disk uses ssd-container (flash mode)
- `secure-boot-vm` uses ssd-container (flash mode)

## Using This Data

### For Unit Tests

```go
func TestVMCollection(t *testing.T) {
    data, _ := os.ReadFile("testdata/vms_list.json")
    var response VMListResponse
    json.Unmarshal(data, &response)

    // Test collector logic
    collector := NewCollector(...)
    vms := collector.parseVMs(response)

    assert.Equal(t, 6, len(vms))
    assert.Equal(t, "web-server-rhel8", vms[0].Name)
}
```

### For Development

Use these files to understand Nutanix API response structure before implementing:
1. Examine field names and types
2. Understand relationships between resources
3. Design Go structs matching the structure
4. Implement mapping logic from API responses to internal models

## UUID Reference

For cross-referencing in tests:

**Clusters**:
- `0005e123-4567-89ab-cdef-000000000001` - prod-cluster-01
- `0005e123-4567-89ab-cdef-000000000002` - dev-cluster-01

**Hosts**:
- `0005f123-4567-89ab-cdef-000000000101` - ahv-node-01
- `0005f123-4567-89ab-cdef-000000000102` - ahv-node-02
- `0005f123-4567-89ab-cdef-000000000201` - ahv-dev-node-01

**VMs**:
- `vm-0005a123-4567-89ab-cdef-000000000001` - web-server-rhel8
- `vm-0005a123-4567-89ab-cdef-000000000002` - db-server-rhel9
- `vm-0005a123-4567-89ab-cdef-000000000003` - win2022-app-server
- `vm-0005a123-4567-89ab-cdef-000000000004` - ubuntu-test-vm
- `vm-0005a123-4567-89ab-cdef-000000000005` - powered-off-vm
- `vm-0005a123-4567-89ab-cdef-000000000006` - secure-boot-vm

**Networks**:
- `0005d123-4567-89ab-cdef-000000000001` - Production-VLAN-100
- `0005d123-4567-89ab-cdef-000000000002` - Production-VLAN-200
- `0005d123-4567-89ab-cdef-000000000003` - Dev-Network
- `0005d123-4567-89ab-cdef-000000000004` - Management-Network

**Storage Containers**:
- `0005c123-4567-89ab-cdef-000000000001` - default-container-prod
- `0005c123-4567-89ab-cdef-000000000002` - ssd-container-prod
- `0005c123-4567-89ab-cdef-000000000003` - default-container-dev

## Notes

- All UUIDs follow the pattern `xxxx-xxxx-xxxx-xxxx-xxxxxx00000N` where N is a sequential number
- UUIDs use different prefixes to identify resource types:
  - `0005e...` - Clusters
  - `0005f...` - Hosts
  - `vm-0005a...` - VMs
  - `0005d...` - Networks
  - `0005c...` - Storage Containers
  - `img-0005b...` - Images

- All timestamps use ISO 8601 format
- Capacities are in bytes, MiB, or GB depending on the field
- AOS version is 6.8.2 across all resources
- All hypervisors are AHV (VM `hypervisor_type`: "AHV"; host
  `hypervisor_full_name`: "Nutanix ...")

## Updating Test Data

When Nutanix API changes or you need additional scenarios:

1. Use the `explore_nutanix.py` script with a real environment
2. Save new responses to this directory
3. Update this README with the new structure
4. Ensure UUIDs remain consistent for referential integrity
