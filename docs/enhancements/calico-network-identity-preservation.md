---
title: calico-network-identity-preservation
authors:
  - "@aaaaaaaalex"
reviewers:
  - "@yaacov"
  - "@mrnold"
approvers:
  - TBD
creation-date: 2026-08-13
last-updated: 2026-08-13
status: implementable
---

# Calico Network Identity Preservation from vSphere

## Release Signoff Checklist

- [ ] Enhancement is `implementable`
- [ ] Design details are appropriately documented from clear requirements
- [ ] Test plan is defined
- [ ] User-facing documentation is created

## Open Questions

1. **Should the NetworkMap field be Calico-named or generic?** This proposal
   adds a `calico` block to `type: pod` entries. An extensible per-CNI block
   (in the spirit of the storage side's `csiVolumeImport`) could hold the
   same fields, with Calico as the first implementation and the existing UDN
   special-casing as a candidate second tenant.
2. **The worked example's Network resource schema.** The `l2Bridge` fields
   shown below track the Calico release that ships the Network resource;
   they will be re-verified against the published schema, and this document
   updated, before the PRs that consume them go up for review.

## Summary

When a VM is migrated from vSphere to a KubeVirt cluster whose pod network is
provided by Calico CNI, the VM should be able to keep its network identity,
and retain L2 connectivity to its source subnet.

An upcoming Calico release will include support for attaching pod NICs to L2
networks, and for requesting L2 and L3 addresses for each of a pod's NICs in
order to preserve workload identity.

This enhancement teaches Forklift to validate Calico resources in the destination,
and also allows Forklift to drive Calico's L2 and address-preservation features via
pod (VM template) annotations.

Users express their intent to use Calico as the networking provider for a given
NIC in the NetworkMap, and Forklift stamps the right annotations on the migrated VM
so it comes up with the same addresses it had on vSphere (assuming validation passed).

## Motivation

Today, for clusters using Calico CNI, a migrated VM will not be able to communicate
over the destination network using their original identity - Calico will assign a fresh
IP for new virt-launcher pods. Additionally, the VMs will be severed from their original
L2 network - instead getting connected to Calico's L3 pod network.

An upcoming Calico release can honour a requested MAC and IP for each of a VM's NICs, and
can attach a pod to a specific VLAN or VRF, but only if the right annotations are present
on the pod (or VM template) when it is created. Forklift can be augmented to stamp those
annotations, and can even go a step further and validate destination cluster config.

### Goals

- Migrated VMs keep their source MAC and IP addresses on Calico-backed
  networks, for (either or both) the primary (pod-network) NIC and secondary (Multus)
  NICs.
- Misconfiguration is caught before migration: broken references, missing
  Calico capability on the destination, addresses that no IPPool can serve —
  all surface as conditions on the NetworkMap or Plan, with messages that say
  what to fix.
- Clusters without Calico, and Calico installations without the newer capabilities,
  are detected; the feature degrades to "not supported here" feedback.
- Ideally, the implementation code is neat and contained:
  - One suggestion was to have no Calico methods on the shared provider interfaces, no stubs in
    unimplemented provider interfaces.
  - There may be an opportunity to create a basic, generic plugin harness with Calico as the
    first implementation.

### Non-Goals

- Sources other than vSphere. The first release validates and preserves
  identity only for vSphere-sourced plans; the NetworkMap rejects the Calico
  opt-in for other source providers with a clear condition.
- In-guest reconfiguration beyond what Forklift already does. The existing
  `preserveStaticIPs` machinery (Windows first-boot re-binding) is reused.
- IPv6.
- Managing Calico itself. Forklift reads Calico resources on the destination;
  it never creates or mutates them, besides stamping an annotation on a migrated VM template.


## A brief look at Calico L2 and L3 networking.

Calico is a K8s networking and policy engine which among other things, ships
with a CNI, and a per-node controlplane daemon. It can be installed in a cluster
for the purpose of networking pods together at Layer 3, and performs IPAM for
said pods ("workloads") when provided with an [IPPool CR](https://docs.tigera.io/calico/latest/reference/resources/ippool).

Traditionally, Calico connects pods together at Layer 3, by transforming each
K8s node into a router. When a CNI ADD event invokes Calico, a virtual
ethernet (veth) device pair is created. One end of the veth in the pod namespace,
and one end in the host namespace. Pod traffic is emitted into the host namespace
where it can be routed, tunneled, etc.

### Address preservation

Historically, if a pod, or Kubevirt VM was scheduled and Calico CNI invoked, a fresh
IP would be allocated from an available Calico `IPPool`. By default, the pod-side link also
receives a semi-random MAC allocated by the kernel. However, these values can
be fixed by annotating the associated Pod manifest:

```yaml
cni.projectcalico.org/hwAddr: "1c:0c:0a:c0:ff:ee"
cni.projectcalico.org/ipAddrs: '["192.168.0.1", "2001:db8::1"]'
```

In Calico Enterprise and Calico Cloud, Multus is officially supported, allowing pods
to receive secondary attachments. An upcoming release will allow setting annotations
for pods' secondary links:

```yaml
cni.projectcalico.org/hwAddr: "1c:0c:0a:c0:ff:ee"
cni.projectcalico.org/ipAddrs: '["192.168.0.1", "2001:db8::1"]'
cni.projectcalico.org/eth1.hwAddr: "1c:2c:2a:c2:ff:ee"
cni.projectcalico.org/eth1.ipAddrs: '["192.168.0.2", "2001:db8::2"]
```

### L2 attachment

The same upcoming release is also scheduled to include Layer 2 pod networking.
Calico L2 expects the cluster admin to expose their K8s nodes to any subnets
they want their pods homed on, i.e., by having a trunk NIC for those networks
plugged ahead of time. Calico can then be configured by way of a `Network` CR
to bridge pod veth traffic onto the trunk, tagged with a VLAN ID.

The `Network` CR is the declarative resource describing what Networks a pod
can be attached to. In the case of L2 bridging, the admin describes in the `Network`:
 - the VLAN,
 - subnet CIDR,
 - the trunk to bridge, and
 - whether they'd like to manage the bridge themselves, or let Calico do it.

In the case where a pod's primary veth should be bridged onto an L2 network known
to Calico, an annotation is placed on the Pod which references the desired `Network`
CR:

```yaml
cni.projectcalico.org/networks: my-network-cr
# A Network can describe many VLANs, so disambiguate if there are many
cni.projectcalico.org/vlan: "123"
```

In the case where a pod's secondary veth should be Calico-bridged in the host-ns,
the reference to the `Network`, the VLAN to use, are instead defined in a NAD's
config (and the pod references the NAD):

```yaml
  apiVersion: k8s.cni.cncf.io/v1
  kind: NetworkAttachmentDefinition
  metadata:
    name: my-nad
    namespace: my-ns
  spec:
    # type "calico" invokes Calico CNI,
    # which reads the Network CR "my-network-cr",
    # searching it for config relating to VLAN 200.
    config: '{
      "cniVersion": "0.3.1",
      "type": "calico",
      "network": "my-network-cr",
      "vlan": 200,
      "ipam": { "type": "calico-ipam" }
    }'

```

### Sample Network resource for an L2-attached Calico node.

```yaml
  apiVersion: projectcalico.org/v3
  kind: Network
  metadata:
    # Cluster-scoped; this is what the `networks`
    # annotation and/or NAD "network" references.
    name: prod-net
  spec:
    l2Bridge:
      # Required: which bridge to use on which nodes, and how it
      # connects to the physical network. The bridge is either
      # pre-existing (managed by the admin) or Calico-managed.
      hostConfig:
        - bridge:
            existingBridge:
              name: br-prod
          hostConnections:
            - trunkPort:
                interface:
                  name: eth1
      # Which VIDs are allowed to pass over the host-ns
      # bridge/trunk upper/lower.
      vlans:
        - vlan:
            id: 100
          subnets:
            # Preserved IPs must fall here, AND inside an enabled
            # IPPool with allowedUses [L2Workload] (exclusive — it
            # cannot be combined with other uses, and the pool must
            # set disableBGPExport) whose CIDR is contained in this
            # subnet.
            - cidr: 10.100.0.0/24
```

_`Network.spec` also has a `vrf` struct which allows for multi-VRF
networking since Calico Enterprise >=v3.23. Docs for that can be
found [here](https://docs.tigera.io/calico-enterprise/latest/networking/configuring/multi-vrf),
though Calico VRF support in Forklift may be omitted from this enhancement
to keep the feature size down._

## Proposal

### The user-facing API

Two additions to NetworkMap items:

**Primary NIC:** A NetworkMap entry of `type: pod` may carry a
`calico` block. Its presence opts the VM's primary NIC into identity
preservation: the source MAC is carried over, and the source IP too when the
Plan sets `preserveStaticIPs`. "Carried over" in the case of a Calico attachment
means that that particular NIC has a corresponding pod-annotation :
`cni.projectcalico.org/<nic>.hwAddr` or `cni.projectcalico.org/<nic>.ipAddrs`.
The optional `network` and `vlan` fields additionally attach the NIC to a VLAN
of a named Calico Network resource; without them the NIC stays on the default
pod network and the preserved IP must fall inside an ordinary workload IPPool.

```yaml
map:
  - source:
      name: "VM Network"
    destination:
      type: pod
      calico:
        network: prod-net   # optional: L2 attach, using the referenced Calico Network resource.
        vlan: 100           # required when network is set.
```

**Secondary NICs (Multus).** No new fields in the NetworkMap. A `type: multus`
entry pointing at a NetworkAttachmentDefinition whose config is `type: calico` and
names a Calico `network` is recognized automatically; the NAD itself holds the opt-in signal.
Forklift preserves the MAC (always) and the IP (when the Plan preserves
static IPs) on that attachment.

```yaml
  - source:
      name: "Backend"
    destination:
      type: multus
      namespace: vms
      name: backend-nad
```

### What Forklift does with Calico attachments

At migration time, the vSphere builder shapes the destination VM:

- The calico-flagged primary NIC uses **bridge binding** instead of
  masquerade, so both the guest and host-namespace see a consistent address.
  The VM template gets the primary-NIC Calico annotations, plus the KubeVirt
  annotation that keeps bridge-on-pod-network VMs live-migratable:

  ```yaml
  cni.projectcalico.org/hwAddr: "00:50:56:aa:bb:01"
  cni.projectcalico.org/ipAddrs: '["10.100.0.5"]'
  cni.projectcalico.org/networks: prod-net
  cni.projectcalico.org/vlan: "100"
  kubevirt.io/allow-pod-bridge-network-live-migration: "true"
  ```

- Each Calico-backed secondary NIC gets the interface-scoped variants, keyed
  by the VMI network name (Calico CNI itself will map pod interface names back to VMI
  network names for virt-launcher pods, as is aware of KubeVirt iface name hashing):

  ```yaml
  cni.projectcalico.org/net-1.hwAddr: "00:50:56:aa:bb:02"
  cni.projectcalico.org/net-1.ipAddrs: '["10.200.0.7"]'
  ```

When the virt-launcher pod is first created, Calico CNI reads the annotations,
reserves the requested address from an eligible IPPool, programs the MAC,
and attaches the interface to the requested segment in the host ns.

### Validation

Validation happens where each concern lives:

- **NetworkMap-scoped**: evaluated by the NetworkMap controller against the destination cluster:
  - the NAD parses and references a Calico Network;
  - the Network exists and is of a usable type;
  - a VLAN is named and exists in the Network;
  - an enabled IPPool with the right `allowedUses` covers the VLAN's subnets;
  - the destination actually ships the needed Calico capability;
  - the `calico` block sits on a `pod` entry and appears at most once.
  Failures are Critical conditions on the map.
- **Plan-scoped**:
  - the interaction with `preserveStaticIPs`,
    - If the plan does not preserve static IPs, and a Calico opt-in is present, Warn. Calico will assign a random IP, and the guest will receive it over DHCP.
      Bigger deal than if Calico were not present, because the opt-in bridges
      the NIC: a DHCP-configured guest adopts the fresh address, while a
      statically-configured guest keeps its old one and diverges from what
      the dataplane routes and enforces.
  - VM placement versus node-scoped networks,
  - a conflict with a UDN-labelled target namespace.
  - per-VM checks:
    - each VM's NIC must have at most one IPv4 to preserve, and:
    - that address must fall inside the VLAN's subnets and be coverable by an eligible IPPool.
    - Failures name the offending VMs in the condition.

### Capability detection

Forklift does not assume the destination's Calico can do any of this. It
probes:

- The `projectcalico.org/v3` **IPPool** resource being served means a
  Calico install whose resources Forklift can read. If it is absent, a
  Calico opt-in is rejected with a condition saying exactly what was
  probed — "the destination does not serve projectcalico.org/v3 IPPools" —
  rather than asserting Calico's absence: an install that serves only the
  internal `crd.projectcalico.org` storage group, or whose aggregated API
  is briefly unavailable (e.g. mid-upgrade), reads as no capability.
- The `projectcalico.org/v3` **Network** resource being served means the
  install ships the per-interface identity and L2/VRF attach capabilities.
  If it is absent, requests that need it are rejected with a condition that
  says so — the user's cluster needs a Calico version that ships the Network
  resource.
- Dataplane requirements are checked per network type:
  - VLAN-bridged networks require the eBPF dataplane;
  - VRF networks require the nftables dataplane.


### Incremental delivery

This lands as a series of self-contained PRs, each reviewable on its own
and each carrying its own unit tests. Later PRs depend only on earlier
ones. PR 2 is deliberately the largest: it banks the whole secondary-NIC
path, which needs no Forklift API change, before the `calico` block lands.

#### PR 1
This document, Calico resources added to the Inventory model, and a hack
script which provisions a kind cluster with some mock Calico CRDs for
Forklift to work off of.

**Inventory:** The provider inventory serves the destination's `projectcalico.org`
resources: IPPools and Networks. Reports on the provider
itself whether the destination serves those resources at all.

**The hack script** Provides a cluster that for all intents and purposes, 
looks like Calico from the perspective of Forklift. Allows us to see how
Forklift will behave in the presence of Calico.

#### PR 2
**CI enablement, and NetworkMap validation for Calico NADs** A CI lane that installs Calico CRDs (or stand-ins)
(see the environment below) and exercises the validation surface end to end.

**NetworkMap validation for Calico NADs:** the NetworkMap-controller checks
for Multus entries backed by Calico NADs, reading Calico state through the
inventory. Broken Calico references surface as conditions on the map.

**Plan validation:** the per-VM address checks for Multus-mapped NICs:
 - at most one IPv4 to preserve,
 - IP is inside the VLAN's subnets,
 - an eligible IPPool covers the subnets.

**Builder:** the interface-scoped annotations
(`cni.projectcalico.org/<net>.hwAddr` / `.ipAddrs`) on the migrated VM.
User-visible result: a Multus-mapped NIC keeps its MAC and IP through a
migration.


#### PR 3
**The `calico` block in a NetworkMap item for primary NIC support.**
The CRD addition and its map-side and plan-side validation (including the
per-VM address checks for the primary NIC).

Stamps metadata to enable bridge binding (so the addresses are consistent everywhere),
the primary NIC address annotations, and the live-migration annotation.

#### PR 4
**VRF network support, if accepted.** Validation for routed (VRF) Calico networks:
route-table conflicts, node coverage versus VM placement, BGP peering and
dataplane checks.

### User Stories

#### Story 1: keep a database server's address

A statically-configured database VM at `10.100.0.5` moves to
KubeVirt. The administrator maps its port group to `type: pod` with
an empty `calico: {}` block and sets `preserveStaticIPs: true` on the Plan.
After cutover the VM answers at `10.100.0.5` with its old MAC.
Since no Calico Network CR was referenced, the NIC was not attached
by Calico during CNI ADD, and rides Calico's L3 pod network.

#### Story 2: a VM lives on the same subnet as K8s nodes.

A statically-configured database VM, and a K8s cluster both live
on the same management subnet, VLAN 100.
The administrator maps the VM port group to `type: pod` with
`calico: {network: mgmt-subnet, vlan: 100}`. They also ensure a thin
IPPool covers the IP of the VM. Calico CNI will create
the primary NIC and bridge it onto the management network with its
original IP and MAC.

#### Story 3: multi-homed appliance

A VM has a management NIC and a backend NIC. The management NIC maps to the
pod network with a `calico` block; the second NIC's NetworkMap entry references
a Multus NAD backed by a Calico Network. Both interfaces keep their MAC and IP;
the plan surfaces a per-VM error before migration if either address cannot be served
by an IPPool on the destination. Otherwise the virt-launcher pod comes up with two
NICs preserving the inner guest's addresses.

#### Story 4: destination can't do it

A user opts a plan into Calico preservation, but the destination's Calico
installation does not support the Network resource. The NetworkMap goes not-ready
with a condition naming the missing capability, before any VM moves.

### Security, Risks, and Mitigations

- The controller gains read-only access to `projectcalico.org` resources
  (`networks`, `ippools`, plus the felix configuration and BGP peer reads
  used by validation). No write access to any Calico resource. The in-repo
  RBAC covers the local (host) destination; a remote destination provider
  needs the same read granted to the provider's credential on that cluster.
- A migrated pod requesting a specific IP could collide with an address in
  use. Mitigations: validation requires the address to be inside an eligible
  IPPool, and Calico's IPAM refuses to double-allocate; the existing MAC
  conflict check warns when a destination VM already uses a source MAC.
- Repeated destination probes use the destination provider's credential.
  All failures are authorization-style (RBAC) rather than
  authentication-style, so they carry no account-lockout risk.

## Design Details

### Worked example

New directory /hack/calico/ holds a script to create a Kind cluster
which can test Forklift's validation of Calico resources. Additionally
included in the dir are Network and IPPool CRDs pulled from Calico EE
(publicly available from [hashreleases](https://2026-08-27-v3-24-2-reentry.hashrelease.tools.tigera.net/)).
The presence of the CRDs satisfies the entire API surface that Forklift
is expected to validate, without the fuss of licensing a flagship
Enterprise installation.

This environment, if desired, can be integrated into the repo's CI for
validation testing.

If testing a full migration is desired (say, to watch Forklift write Calico annotations),
this same hack can be applied in a KubeVirt-enabled cloud cluster.

Where the hack falls short is in Calico's processing of the CRDs.
Since the Calico agent is still OSS, it will not process the annotations. That being said, this
falls well outside the realm of Forklift's responsibilities, and any failure
by Calico to read annotations or facilitate the proper setup of a pod should
be filed as an issue on github.com/projectcalico/calico.

### Test Plan

- **Unit tests** accompany every PR: the Calico client (resource parsing,
  IPPool eligibility), the NetworkMap validation (each failure mode as a
  condition), the plan-side per-VM checks, and the builder's annotation
  output for primary and secondary paths.
- **Demo evidence**: each behaviour-adding PR includes the YAML applied and
  the resulting objects/logs from a real migration run in our lab, so
  reviewers see expected output without needing a vSphere source.
- **CI**: the environment above is scriptable end to end (kind + operator
  manifests + KubeVirt with emulation). We will contribute a CI lane that
  builds it and exercises the validation surface; the identity assertions
  that need the Network resource are gated on the capability probe so the
  lane degrades gracefully on installs that do not ship it.

### Upgrade / Downgrade Strategy

The `calico` block is a new optional field: existing NetworkMaps are
unaffected, and a downgraded controller ignores the field (the CRD keeps
it). No stored data migrates. Removing the opt-in from a map returns the
next migration to today's behaviour (masquerade, fresh addresses).

## Implementation History

- 2026-08-13: enhancement proposed. A complete implementation exists on the
  author's fork (previously #6936) and is being re-submitted in the
  increments listed above.

## Drawbacks

- Bridge binding on the pod network trades some of masquerade's isolation
  for identity: the guest owns the pod's real address. This is opt-in and
  the trade is the point of the feature, but it is a behavioural difference
  worth documenting for users.

## Alternatives

- **A generic CNI plugin field instead of `calico`.** The NetworkMap could
  carry an extensible per-CNI block (similar in spirit to the storage side's
  `csiVolumeImport`), with Calico as the first implementation and the
  existing UDN special-casing as a candidate second tenant. The maintainers'
  interest in a plugin-style shape is noted; the current proposal keeps the
  field Calico-named for clarity but the internal layout (self-contained
  validation called from fixed points, no provider-interface methods) is
  deliberately plugin-ready. This can be revisited at PR 3 before the CRD
  field lands.
- **Probing with a destination client instead of the inventory.** The
  previous submission read Calico resources directly from the destination
  cluster at validation time, fetching and discarding on every pass.
  Reviewer feedback on that submission asked for the inventory to carry
  these reads, so this proposal makes the inventory the single read path
  for Calico state (PR 1) and later PRs consume it from there.
- **Doing nothing**: users keep reconfiguring migrated VMs by hand, or
  pinning addresses by pre-creating pods — neither survives scale or
  auditing.

## Infrastructure Needed

- **A CI lane with Calico installed.** The project's CI does not currently
  install Calico, so none of this feature's validation surface can run
  there today. The kind-based environment above is fully scriptable
  (kind + Calico operator + Multus + KubeVirt with emulation); we will
  contribute the setup scripts and the capability-gated test suite, and
  will need a maintainer's help wiring the lane into the project's CI and
  deciding where it runs (per-PR or periodic). Tracked as PR 2 of the
  incremental plan.
- **No new repositories or subprojects.** All code lands in-tree; the
  Calico client is a small read-only package under `pkg/lib/client`.
