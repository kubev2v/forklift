---
title: nad-pool-mapping
authors:
  - "@yaacov"
reviewers:
  - TBD
approvers:
  - TBD
creation-date: 2026-05-10
last-updated: 2026-05-10
status: implementable
---

# NAD Pool Mapping

## Release Signoff Checklist

- [x] Enhancement is `implementable`
- [x] Design details are appropriately documented from clear requirements
- [x] Test plan is defined
- [ ] User-facing documentation is created

## Summary

VMs with multiple NICs attached to the same source network cannot be migrated
today because each source network maps to exactly one destination NAD
(Network Attachment Definition). OVN-Kubernetes uses NAD names as map keys, so
when two NICs resolve to the same NAD, the migration is rejected with a
`VMDuplicateNADMappings` validation error.

This enhancement allows a source network to map to multiple destination NADs
(NAD pool). When building the KubeVirt VM spec, each NIC on the same source network
is assigned a distinct NAD from the pool, eliminating duplicates.

### Goals

* Allow multiple `NetworkPair` entries in a `NetworkMap` to share the same
  source network with different destinations, enabling NAD pool mapping.
* Ensure no NAD is assigned twice on the same VM.
* Maintain full backward compatibility: single-row maps behave identically to
  today.

### Non-Goals

* No CRD schema change. The existing `NetworkMap` format is reused as-is.
* This enhancement does not address automatic NAD creation or discovery.

## Motivation

A posible vSphere configuration is a VM with multiple NICs on the same
distributed port group (e.g., a database server with separate NICs for
application traffic and replication, both on the same network). Today, Forklift
blocks migration of these VMs because the `NetworkMap` only supports 1:1
source-to-NAD mapping.

### Example: Creating a dual-NIC VM on vSphere using govc

#### Find the portgroup used by a template VM

```bash
govc vm.info -json template-vm | jq -r '.virtualMachines[0].config.hardware.device[] | select(.deviceInfo.label | contains("Network")) | .backing.deviceName'
```

#### Clone the VM

```bash
govc vm.clone -vm template-vm -ds <datastore> dual-nic-vm
```

#### Add a second NIC to the same portgroup

```bash
# vmxnet3 is the adapter type (e1000, e1000e, vmxnet2, vmxnet3)
# vmxnet3 is recommended for best performance
govc vm.network.add -vm dual-nic-vm -net "<portgroup>" -net.adapter vmxnet3
```

#### Verify dual NIC configuration

```bash
govc device.ls -vm dual-nic-vm | grep -i network
```

### Example: Creating two NADs for the same network

Both NADs join the same OVN-Kubernetes layer2 segment but have distinct names
so that each NIC on the VM gets its own attachment.

```bash
oc apply -f - <<'EOF'
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: nad-a
  namespace: target-ns
spec:
  config: |
    {
      "cniVersion": "1.0.0",
      "name": "nad-a",
      "type": "ovn-k8s-cni-overlay",
      "topology": "layer2",
      "subnets": "10.10.0.0/24",
      "netAttachDefName": "target-ns/nad-a"
    }
---
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: nad-b
  namespace: target-ns
spec:
  config: |
    {
      "cniVersion": "1.0.0",
      "name": "nad-b",
      "type": "ovn-k8s-cni-overlay",
      "topology": "layer2",
      "subnets": "10.10.0.0/24",
      "netAttachDefName": "target-ns/nad-b"
    }
EOF
```

Verify both NADs exist:

```bash
oc get net-attach-def -n target-ns
```

Without NAD pool mapping, migrating `dual-nic-vm` fails with
`VMDuplicateNADMappings`.

## Proposal

### User flow

**Prerequisites:** Ensure source (vSphere) and target (host) providers are created and ready:

```bash
# Create vSphere source provider
oc mtv create provider \
  --name my-vsphere \
  --type vsphere \
  --url https://vcenter.example.com/sdk \
  --username admin@vsphere.local \
  --password 'secret' \
  --provider-insecure-skip-tls \
  -n konveyor-forklift

# Create host target provider (local OpenShift cluster)
oc mtv create provider \
  --name host \
  --type openshift \
  -n konveyor-forklift

# Wait for providers to be ready
oc wait --for=condition=Ready provider/my-vsphere -n konveyor-forklift --timeout=180s
oc wait --for=condition=Ready provider/host -n konveyor-forklift --timeout=120s
```

1. List source networks to find the portgroup ID:

   ```bash
   oc mtv get inventory network --provider my-vsphere -n konveyor-forklift
   ```

2. Create a Plan with inline network pairs mapping the same source network to
   multiple NADs. The `--network-pairs` flag uses the format `<source>:<destination>`:

   ```bash
   oc mtv create plan \
     --name dual-nic-plan \
     --source my-vsphere \
     --target host \
     --vms dual-nic-vm \
     --network-pairs "VM Network:target-ns/nad-a,VM Network:target-ns/nad-b" \
     -n konveyor-forklift
   ```

   **Alternative:** Create a separate `NetworkMap` and reference it in the plan:

   ```bash
   # Create NetworkMap
   oc mtv create mapping network \
     --name dual-nic-netmap \
     --source my-vsphere \
     --target host \
     --network-pairs "VM Network:target-ns/nad-a,VM Network:target-ns/nad-b" \
     -n konveyor-forklift

   # Create Plan referencing the NetworkMap
   oc mtv create plan \
     --name dual-nic-plan \
     --source my-vsphere \
     --target host \
     --vms dual-nic-vm \
     --network-map dual-nic-netmap \
     -n konveyor-forklift
   ```

   Verify the plan is ready (no `VMDuplicateNADMappings`):

   ```bash
   oc mtv get plan -n konveyor-forklift
   oc wait --for=condition=Ready plan/dual-nic-plan -n konveyor-forklift --timeout=120s
   ```

3. Start the migration:

   ```bash
   oc mtv start plan --name dual-nic-plan -n konveyor-forklift
   ```

   During migration, NIC-1 is assigned `nad-a` and NIC-2 is assigned `nad-b`.

4. Monitor progress:

   ```bash
   oc mtv get plan --name dual-nic-plan --vms -n konveyor-forklift
   ```

### Implementation overview

The implementation extracts all NAD pool complexity into new helper methods,
keeping existing provider `mapNetworks` functions as unchanged as possible.

**New types and helpers:**

* `NADPool` (in `pkg/controller/plan/adapter/base/network.go`) -- a
  stateful object created once per VM that tracks which Multus NADs have been
  assigned. Exposes an `Allocate(candidates)` method that picks the first
  unused NAD from a list of Multus-only candidates.

* `AllocateNetwork(pool, pairsForSource)` (in `base/network.go`) -- top-level
  routing function that callers use instead of `pool.Allocate` directly.
  Non-Multus destinations (pod, ignored) pass through immediately; Multus
  destinations are forwarded to the `NADPool` for deduplication.

* `FindAllNetworks`, `FindAllNetworksByType`, `FindAllNetworksByNameAndNamespace`
  (on `NetworkMap` in `pkg/apis/forklift/v1beta1/mapping.go`) -- return all
  matching pairs instead of just the first.

* `FindAllMappingsForNICRef` (in `base/network.go`) -- delegates to the
  `FindAll*` methods above.

**Per-provider changes:**

Each provider that uses NAD pool mapping builds a `buildNICResolver` method
that returns `(nicKeys []string, pairsBySource map[string][]NetworkPair)`.
The caller loops over NICs and calls
`AllocateNetwork(pool, pairsBySource[nicKeys[i]])`. The only difference
between providers is how each maps source inventory to index keys.

| Provider | NIC key | How map entries are indexed |
|----------|---------|---------------------------|
| vSphere | `nic.Network.ID` | Inventory lookup; indexed by `network.Key` (if variant matches) and `network.ID` |
| Hyper-V | `nic.Network.ID` | Inventory lookup; indexed by `network.ID` |
| oVirt | `nic.Profile.Network` | Inventory lookup; indexed by `network.ID` |
| OVA/OVF | `nic.Network` | Inventory lookup; indexed by `network.Name` |
| EC2 | `*eni.SubnetId` | Direct field; indexed by `Source.ID` and `Source.Name` |

**Providers excluded from NAD pool mapping:**

* **OpenStack** -- `vm.Addresses` is a `map[string]interface{}` keyed by
  network name. Since map keys are unique, a VM cannot have two separate NICs
  on the same source network. The duplicate-NAD scenario that NAD pool mapping
  solves cannot arise, so OpenStack keeps its original first-match lookup with
  no pool allocation.

* **OCP live migrator** -- OCP-to-OCP migration maps NADs directly to NADs
  (1:1). There is no "source network" that multiple NICs could share, so the
  duplicate-NAD problem does not apply.

**Validation:**

`ValidateNetworkDuplicates` now uses `AllocateNetwork` with a temporary
`NADPool` to simulate assignment. Duplicates are flagged only when the NAD
pool for a source network is exhausted (NIC count exceeds available NADs).
The function signature is unchanged, so `validation.go` needs zero changes.

**NetworkMap controller:**

`validateSource` deduplicates `status.refs` entries so that multiple map rows
with the same source do not produce duplicate refs.

### Security, Risks, Mitigations and Limitations

* **NAD pool exhaustion**: If a VM has more NICs on a source network than
  available NADs, validation catches this and blocks the plan with
  `VMDuplicateNADMappings`, the same condition as today.

* **NAD configuration mismatch**: The user is responsible for ensuring that
  all NADs in the pool are configured for the same logical network. Forklift
  does not validate L2/L3 equivalence between NADs.

* **Ordering**: NADs are assigned to NICs in the order they appear in
  `spec.map` and the order NICs appear in the VM inventory. This is
  deterministic but not user-configurable.

## Design Details

### Backward compatibility

* A `NetworkMap` with one row per source network works identically to today:
  the pool returns the single match, same as `FindMappingForNICRef`.
* `ValidateNetworkDuplicates` return type and semantics are unchanged.
* The oVirt/OVA `resolveNICMappings` helper produces identical output when
  sources are unique (each NIC still maps to exactly one entry).

### Test Plan

**Unit tests** (in `pkg/controller/plan/adapter/base/network_test.go`):

* `FindAllMappingsForNICRef` -- multiple matches by ID, single match, nil map,
  no match.
* `NADPool` -- distinct NADs assigned from pool, pool exhaustion,
  independent networks, empty candidates.
* `AllocateNetwork` -- pod passthrough, Multus delegation to pool.
* `ValidateNetworkDuplicates` with NAD pool -- no duplicate when pool is
  sufficient, pool exhaustion flagged, mixed networks, backward-compatible
  single-row behavior.

**E2E test scenarios:**

* Migrate a vSphere VM with 2 NICs on the same portgroup using a `NetworkMap`
  with 2 entries for that portgroup. Verify the resulting KubeVirt VM has 2
  distinct Multus networks.
* Migrate the same VM with only 1 NAD in the map. Verify validation blocks the
  plan with `VMDuplicateNADMappings`.
* Migrate a VM with NICs on different source networks (1:1 per network).
  Verify no behavioral change from current behavior.

### Upgrade / Downgrade Strategy

No migration of existing resources is needed. Existing `NetworkMap` CRs with
unique sources per row continue to work without changes. The new behavior only
activates when multiple rows share the same source.

### Open Questions

None at this time.

## Alternatives

**Alternative 1: Add a `Destinations []DestinationNetwork` field to
`NetworkPair`.**

This would make the NAD pool intent explicit in the CRD schema but requires a CRD
schema change, deepcopy regeneration, and migration of existing resources. The
chosen approach avoids all of this by reusing the existing slice-of-pairs
format.

**Alternative 2: Do nothing.**

Users with dual-NIC VMs on the same network would need to manually reconfigure
VMs before migration (remove a NIC, migrate, re-add), which is error-prone and
impractical at scale.

## Acceptance

| Role               | Name | Date | Decision              |
|--------------------|------|------|-----------------------|
| MTV Arch Member    |      |      | Accept / Reject (reason) |

## Implementation History

* 05/10/2026 - Enhancement submitted.
