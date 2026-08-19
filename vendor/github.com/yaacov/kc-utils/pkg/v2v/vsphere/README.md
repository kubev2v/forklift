# vsphere -- govmomi vSphere connection and disk discovery

Connects to a vCenter server via the govmomi SDK and queries VM metadata needed for migration: disk VMDK paths, NIC MAC addresses, firmware type, and guest OS hints. This data feeds into the kc-v2v copy and prepare stages.

The `LoadInventory` function parses the libvirt-style URL from `V2V_libvirtURL` to extract the vCenter host and datacenter, reads credentials from Forklift secret files, and establishes an authenticated govmomi session with TLS from `pkg/v2v/tls` (`ForkliftTLS`, `VCenterConfig`, optional `SetThumbprint` via `V2V_fingerprint`). It then retrieves the VM's hardware device list and extracts disk paths (following snapshot chains to base VMDKs), NIC MAC addresses (across vmxnet3, e1000, PCNet32, and SR-IOV types), firmware type, and guest ID/name. Results are cached in-process so that both the copy and conversion phases reuse the same inventory without reconnecting.

Disk ordering follows the libvirt bus priority (SCSI > SATA > IDE > NVMe), then sorts by controller key and unit number. Snapshot delta VMDKs are resolved back to their base disk filenames by walking the backing chain and stripping `-NNNNNN.vmdk` suffixes.

## File layout

| File | Purpose |
|------|---------|
| `connect.go` | `sdkURL`, `credentials`, `connect`, `ConnectHost`, `datacenterName` -- vCenter URL parsing, secret-based auth, govmomi client with TLS policy and thumbprint fallback |
| `disks.go` | `disksFromDevices` and helpers -- extracts and sorts VMDK paths from VM device list, resolves snapshot chains |
| `inventory.go` | `Inventory`, `LoadInventory`, `ResetCache` -- top-level VM metadata query with in-process caching |

## Key exports

| Symbol | Role |
|--------|------|
| `Inventory` | Struct holding VM moref, disk paths, NICs, firmware hint, guest ID/name, and hostname |
| `LoadInventory` | Queries vCenter for VM metadata given a `*config.Config`; results are cached per URL+VM+fingerprint key |
| `ConnectHost` | Connects to `https://host/sdk` with `v2vtls.Policy` and vCenter fingerprint |
| `ResetCache` | Clears the inventory cache (used in tests) |
