---
title: guest-conversion-security-profiles
authors:
  - "@lwr20"
  - "@aaaaaaaalex"
reviewers:
  - TBD
approvers:
  - TBD
creation-date: 2026-09-14
last-updated: 2026-09-14
status: implementable
---

# Guest Conversion Security Profiles

## Release Signoff Checklist

- [] Enhancement is `implementable`
- [] Design details are appropriately documented from clear requirements
- [] Test plan is defined
- [] User-facing documentation is created

## Summary

The final step of a migration is guest conversion: a pod runs virt-v2v, which
boots a small libguestfs appliance. The appliance gets its
networking through passt, and passt sandboxes itself: It creates a user
namespace and remounts its filesystem view before it will carry any traffic.
On non-Openshift K8s clusters the node's default confinement can stop that sandbox, so
migrations fail at the conversion step: disks transfer fine, then conversion
dies with "Couldn't create user namespace: Operation not permitted".

This enhancement lets the cluster administrator name the seccomp and AppArmor
profiles that conversion pods run under, and ships profiles that permit exactly
what passt's sandbox needs, along with an opt-in way to place them on nodes:
a DaemonSet at `hack/seccomp/profile-installer.yaml`.

## Motivation

Two independent barriers cause the failure:

1. The container runtime's default seccomp profile allows `unshare(2)` only for
   callers holding `CAP_SYS_ADMIN`, and the conversion pod runs unprivileged
   with all capabilities dropped.
2. On distributions that enforce AppArmor (Ubuntu, SUSE), the runtime's default
   AppArmor profile denies `mount`.

Clearing the first barrier only surfaces the second. Different node operating
systems face different subsets: AppArmor-enforcing distributions hit both
barriers, SELinux-based distributions hit only the seccomp one, and OpenShift
hits neither because its nodes ship a seccomp profile that permits `unshare`
and confinement there is SELinux.

The existing code applies a custom (Localhost) seccomp profile to the
conversion pod only on OpenShift. That guard exists for a good reason: naming a
Localhost profile that a node does not carry makes the kubelet refuse to start
the pod at all (#1942) — a worse failure than the conversion error. But it
leaves conversion impossible everywhere else.

Privilege is not an escape hatch. Even a fully privileged conversion pod fails
the same way, because passt drops root before it sandboxes itself, so it no
longer holds `CAP_SYS_ADMIN` at the moment it needs it. The privileged-pod
workaround circulating on #4491 was tested and does not work.

Affected users include #4997 (vanilla Kubernetes 1.33 with a vSphere source)
and #4491 (Harvester) — in general, any containerd-based cluster.

### Goals

* Migrations complete on clusters whose default confinement blocks passt's
  sandbox, once the administrator names suitable profiles.
* Confinement doesn't change when upgrading: unset settings mean today's
  behavior exactly.
* Ship working profiles and an opt-in installer for clusters with no other way
  to place profiles on nodes.

### Non-Goals

* Removing the appliance's passt dependency in libguestfs/virt-v2v.
* Installing profiles onto nodes as part of the default install.
* Changing the confinement of any pod other than those that boot the appliance.

## Proposal

Four new fields on the ForkliftController spec:

* `virt_v2v_seccomp_profile_type` — `RuntimeDefault`, `Localhost`, or
  `Unconfined`.
* `virt_v2v_seccomp_profile_path` — a path below the kubelet's seccomp root;
  defaults to `profiles/unshare.json` when the type is `Localhost`.
* `virt_v2v_apparmor_profile_type` — `RuntimeDefault`, `Localhost`, or
  `Unconfined`.
* `virt_v2v_apparmor_profile_path` — the name of an AppArmor profile already
  loaded on the node; defaults to `forklift-virt-v2v-unshare` when the type is
  `Localhost`.

The same knobs exist as controller environment variables
(`VIRT_V2V_SECCOMP_PROFILE_TYPE` and so on). Supplying a path without its
corresponding `Localhost` type is rejected at admission.

All four default to unset, so an upgrade doesn't smuggle a behavioural change.
OpenShift keeps the profile it already had, every other cluster keeps `RuntimeDefault`.

The named profiles apply to every pod that boots the appliance: the conversion
pod and the deep-inspection pod.

This proposal also ships the two profiles: a seccomp profile that
permits unprivileged `unshare`, and an AppArmor profile that permits the
remount passt performs. Finally an installer DaemonSet under
`hack/seccomp/`, for clusters with no other way to place profiles on nodes.
This last bit may be outside the remit of Forklift - please advise.
Where the Security Profiles Operator is already in use, it is the preferred
delivery mechanism. A new document, `docs/guest-conversion-seccomp.md`,
explains the two barriers and how to tell which of them a given node enforces.

The pod-level AppArmor field requires Kubernetes 1.30 or newer; on older
clusters only the seccomp settings are usable.

### Security, Risks, and Mitigations

The shipped seccomp profile widens the runtime default only by allowing
unprivileged `unshare`; the AppArmor profile is scoped to what passt's sandbox
does. Administrators can point at their own stricter profiles instead.

Misconfiguration fails closed: naming a profile the node does not carry means
the kubelet refuses to start the pod, rather than the pod running unconfined.
Because the settings default to unset, upgrading changes no pod's confinement.

## Design Details

### Test Plan

Verified end-to-end on a live RKE2 v1.33.5 cluster (Ubuntu 24.04, containerd
2.1.4, AppArmor enforcing) with a vSphere source:

* The baseline failure was reproduced with the real passt binary under the
  runtime default profile.
* The two barriers were shown to be independent: the seccomp profile alone
  moved the failure from the user-namespace step to the mount step; with both
  profiles named, passt completed its sandbox.
* A full migration then completed — the image-conversion step that previously
  always failed succeeded.

Unit tests cover the settings parsing and validation, profile resolution on
each platform, and that the resolved profiles actually reach the pod specs.

### Upgrade / Downgrade Strategy

No action is needed on upgrade: with the settings unset, every cluster keeps
the behavior it has today. To use the enhancement, place the profiles on the
nodes, then set the fields. On downgrade, remove the settings and conversion
pods revert to the default profiles. On Kubernetes older than 1.30 the
AppArmor settings cannot be applied; the seccomp settings still work, but
on past-EoL K8s versions, migration will probably fail if AppArmor gets in
the way.

## Drawbacks

Delivering profiles to nodes is inherently outside Forklift's normal
footprint. The installer DaemonSet is therefore opt-in and lives under
`hack/`, not in the default install.

## Alternatives


* ~~Run conversion pods privileged~~. Demonstrated not to work: passt drops root
  before it sandboxes itself.

## Implementation History

* Sept 15 2026 - Enhancement submitted.
