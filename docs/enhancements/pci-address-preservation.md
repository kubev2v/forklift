---
title: pci-address-preservation
authors:
  - "@yaacov"
reviewers:
  - TBD
approvers:
  - TBD
creation-date: 2026-06-14
last-updated: 2026-06-23
status: implementable
see-also:
  - "https://redhat.atlassian.net/browse/MTV-5753"
---

# PCI Address Preservation

## Release Signoff Checklist

- [x] Enhancement is `implementable`
- [x] Design details are appropriately documented from clear requirements
- [x] Test plan is defined
- [ ] User-facing documentation is created

## Summary

MTV collects PCI slot information from source vSphere VMs and converts it to
standard PCI addresses at inventory time. This enables downstream consumers
(plan builder, CLI, UI) to preserve PCI topology when migrating VMs to KubeVirt.

## Motivation

KubeVirt supports setting PCI addresses on NIC interfaces. By collecting and
converting PCI slot numbers from the source VM, MTV can preserve the original
PCI topology across migration.

### Goals

* Collect PCI slot numbers from vSphere VM device configuration.
* Collect PCI bridge topology from `config.extraConfig`.
* Convert slot numbers to standard PCI addresses at inventory time using
  the bridge-aware algorithm.
* Store the PCI address on the `NIC` struct for downstream use.
* Store the raw PCI slot number and device key on the `Device` struct to
  mirror vSphere's data model.

### Non-Goals

* Builder/migration integration (setting `pciAddress` on target KubeVirt VMI
  interfaces) is a follow-up enhancement.
* Support for providers other than vSphere.

## Design

### Data Model

**`Device` struct** — one entry per virtual hardware device (NIC, disk controller,
etc.):

| Field           | Type    | Description                               |
|:----------------|:--------|:------------------------------------------|
| `Key`           | `int`   | vSphere device key (e.g. 4000)            |
| `Kind`          | `string`| Device type (e.g. `VirtualVmxnet3`)       |
| `PciSlotNumber` | `int32` | Raw vSphere PCI slot number               |

**`NIC` struct** — one entry per network adapter:

| Field       | Type     | Description                                     |
|:------------|:---------|:------------------------------------------------|
| `DeviceKey` | `int`    | Links to the corresponding `Device.Key`         |
| `MAC`       | `string` | MAC address                                     |
| `PciAddress`| `string` | Computed PCI address (`DDDD:BB:DD.F`)           |
| `Network`   | `Ref`    | Reference to the network                        |
| `Order`     | `int`    | NIC order on the VM                             |

**`PciBridge` struct** — one entry per PCI bridge parsed from `config.extraConfig`:

| Field        | Type    | Description                                    |
|:-------------|:--------|:-----------------------------------------------|
| `Number`     | `int`   | Bridge index (e.g. 0, 4, 5, 6, 7)             |
| `SlotNumber` | `int32` | Bridge's own PCI slot on bus 0                 |
| `Functions`  | `int`   | Number of root-port functions (buses provided) |

### vSphere PCI Slot Number Encoding

vSphere assigns each virtual device a **PCI slot number** — a persistent integer
stored at `config.hardware.device[].SlotInfo.PciSlotNumber`. The slot number is
a packed bitmap with the layout `FFF.BBBBB.DDDDD` (Broadcom KB 311606):

```
DDDDD = slot & 0x1F          (bits 0–4)   device on the secondary bus
BBBBB = (slot >> 5) & 0x1F   (bits 5–9)   bridge index: 0 = primary, N = pciBridge[N-1]
FFF   = (slot >> 10) & 0x7   (bits 10–12) root-port function within the bridge device
```

All PCIe devices (VMXNET3, PVSCSI, e1000e) have `BBBBB != 0` — they sit behind
PCIe root ports. Legacy PCI devices (e1000, lsilogic) have `BBBBB == 1`
(behind pciBridge0, the legacy PCI-to-PCI bridge).

### Bridge Topology

The PCI bridge configuration is stored in the VM's `config.extraConfig`:

```
pciBridge0.pciSlotNumber = "17"     (legacy PCI bridge, 1 bus)
pciBridge4.pciSlotNumber = "21"     (PCIe root port)
pciBridge4.functions = "8"          (8 root-port functions = 8 buses)
pciBridge5.pciSlotNumber = "22"
pciBridge5.functions = "8"
pciBridge6.pciSlotNumber = "23"
pciBridge6.functions = "8"
pciBridge7.pciSlotNumber = "24"
pciBridge7.functions = "8"
```

Bridges without an explicit `functions` entry default to 1 function (1 bus).

### Conversion Algorithm

1. Parse all `pciBridgeN.pciSlotNumber` and `pciBridgeN.functions` entries from
   `config.extraConfig`.
2. Sort bridges by their device number on bus 0 (`bridgeSlot & 0x1F`).
3. Assign bus numbers sequentially starting from `pciBusBaseOffset = 2`
   (bus 0 = root complex, bus 1 = AGP bridge in the I440BX chipset):
   - pciBridge0 (legacy PCI, 1 function): bus 2
   - pciBridge4 (PCIe, 8 functions): buses 3–10
   - pciBridge5 (PCIe, 8 functions): buses 11–18
   - pciBridge6 (PCIe, 8 functions): buses 19–26
   - pciBridge7 (PCIe, 8 functions): buses 27–34
4. For a device with slot S:
   - If `BBBBB == 0`: device is on bus 0 → `0000:00:DDDDD.0`
   - Otherwise: find pciBridge[BBBBB-1], compute `bus = bridge.baseBus + FFF`,
     `dev = DDDDD` → `0000:bus:dev.0`

If the bridge topology is not available in `config.extraConfig`, `pciAddress`
is left empty.

**Chipset assumption:** `pciBusBaseOffset = 2` assumes the I440BX virtual chipset
where bus 1 is the AGP bridge at `00:01.0`. All VMware hardware versions
(vmx-04 through vmx-21, ESXi 2.x–8.x) use I440BX for both BIOS and EFI firmware.

### Worked Examples

Verified against `lspci` output inside the guest OS:

| vSphere Slot | FFF | BBBBB | DDDDD | Bridge     | Guest Bus | PCI Address  |
|:------------:|:---:|:-----:|:-----:|:----------:|:---------:|:------------:|
| 33           | 0   | 1     | 1     | pciBridge0 | 2         | 0000:02:01.0 |
| 160          | 0   | 5     | 0     | pciBridge4 | 3         | 0000:03:00.0 |
| 192          | 0   | 6     | 0     | pciBridge5 | 11        | 0000:0b:00.0 |
| 224          | 0   | 7     | 0     | pciBridge6 | 19        | 0000:13:00.0 |
| 256          | 0   | 8     | 0     | pciBridge7 | 27        | 0000:1b:00.0 |
| 1184         | 1   | 5     | 0     | pciBridge4 | 4         | 0000:04:00.0 |

### Collection Flow

1. The vSphere inventory collector fetches `config.hardware.device` and
   `config.extraConfig` via the govmomi property collector.
2. `config.extraConfig` entries matching `pciBridgeN.*` are parsed into
   `PciBridge` structs and stored on the `VM` model.
3. Each device's `PciSlotNumber` and `Key` are stored in the `Device` list.
4. For each NIC, the collector matches the NIC's backing device key to a
   `Device`, calls `computePciAddress(slot, bridges)` to derive the PCI
   address, and stores it on the `NIC` struct.

## Alternatives

**Alternative 1: Store only the raw slot number, convert at use-time.**

This avoids storing a derived value but pushes conversion logic into every
consumer (builder, CLI, UI). Storing the pre-converted address is simpler for
downstream consumers.

**Alternative 2: Store only the PCI address, not the raw slot.**

This loses the original vSphere value, making it harder to debug mismatches or
handle future edge cases. Storing both preserves full fidelity.
