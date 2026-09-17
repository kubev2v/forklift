---
title: target-virtualization-validation-mvp
authors:
  - "@tiraboschi"
creation-date: 2026-09-17
last-updated: 2026-09-17
status: provisional
see-also:
  - "/enhancements/target-virtualization-validation-proposal.md"
  - "https://github.com/kubev2v/forklift/pull/8507"
  - "https://github.com/openshift-cnv/virt-cluster-validate/pull/30"
---

# Target Virtualization Validation MVP

## Summary

Deliver a small, end-to-end validation workflow for a target OpenShift
Virtualization cluster. A `VirtualizationValidation` custom resource (CR) on
the Forklift hub creates a validator Job in the Forklift namespace. The Job
uses the referenced OpenShift Provider credentials to run read-only
`virt-cluster-validate` checks against the remote target and progressively
publishes results to a controller-created ConfigMap. The Forklift controller
projects a compact current summary into the referenced Provider status.

The CR is the durable detailed run record. The Provider is only a compact
summary surface for existing API consumers and the console.

This MVP is implemented as two coordinated pull requests: one upstream in
`openshift-cnv/virt-cluster-validate` (VCV), and one in Forklift.

## Goals

* Prove the full hub-local flow: CR -> Job -> remote target checks -> partial
  results -> CR and Provider status.
* Use an OpenShift Provider's existing URL, bearer token, and TLS settings;
  do not require a Forklift CR or a service account on the remote target.
* Make a validation visible from a Provider without changing normal Provider
  behavior by default.
* Establish a safe, versioned machine-result contract before adding
  Plan-aware workload probes.

## Non-goals

* Automatic validation when a Provider is created or updated.
* Blocking Provider readiness for ordinary failed checks.
* Creating VMs, PVCs, snapshots, migrations, or other workload resources on
  the remote target.
* Plan-created validations or Plan readiness/execution gating.
* Remote Job execution, remote result artifacts, or remote cleanup logic.
* User-configurable images, arbitrary commands, or arbitrary check paths.

## MVP architecture

```text
VirtualizationValidation CR                 OpenShift Provider Secret
            |                                        |
            | owns and watches                       | mounted read-only
            v                                        v
    Forklift validation controller ---> hub-local validator Job
            ^                                        |
            | watches ConfigMap and Job              | URL/token authentication
            |                                        v
      result ConfigMap <--- CTRF snapshots --- remote target API
```

The Job runs in the Forklift controller namespace, normally `openshift-mtv`.
It is not a workload on the remote target. This intentionally keeps the
Provider credential in the same administrative trust domain as Forklift,
rather than copying it to a spoke cluster.

## API contract

Add `VirtualizationValidation` to `forklift.konveyor.io/v1beta1`.

```yaml
apiVersion: forklift.konveyor.io/v1beta1
kind: VirtualizationValidation
metadata:
  name: target-ocp-readiness
  namespace: openshift-mtv
spec:
  providerRef:
    name: target-ocp
  profile: Provider
  checks:
    - platform
  runNonce: "1"
status:
  phase: Running
  observedInputHash: sha256:...
  jobRef:
    name: virtualization-validation-...
    namespace: openshift-mtv
  resultRef:
    name: virtualization-validation-...
    namespace: openshift-mtv
  image: quay.io/.../forklift-virt-validation@sha256:...
  summary:
    total: 24
    pending: 4
    passed: 19
    failed: 1
    skipped: 0
```

Action items:

* Define `ProviderRef`, profile, requested catalog checks, and `RunNonce` in
  the spec. Require an OpenShift Provider.
* Reserve profile values `Provider` and `Plan`; the MVP controller accepts
  only `Provider` and reports `Plan` as unsupported.
* Do not expose image, shell commands, raw VCV include patterns, target
  namespace, or target service account in the user API.
* Define `Pending`, `Running`, `Succeeded`, and `Failed` phases, standard
  conditions, Job/ConfigMap references, image identity, input hash, timestamps,
  bounded summary, and bounded per-check outcomes.
* Calculate the input hash from Provider identity/type/URL, referenced Secret
  resource version, profile/catalog, selected image digest, and `runNonce`.
  Never hash or expose Secret data.
* Make the CR own its hub-local Job, ConfigMap, Role, and RoleBinding.
* Generate deepcopy and CRD artifacts, add a sample, and grant viewer/editor
  access to the new resource and its status.

## Provider status projection

When a `VirtualizationValidation` references a Provider, the controller
projects its latest result into that Provider's top-level conditions. The CR
remains the source of detailed and historical evidence.

* `VirtualizationValidationInProgress`: advisory; identifies the active CR.
* `VirtualizationValidationSucceeded`: normal/advisory; records the CR,
  effective input hash, and completion time.
* `VirtualizationValidationFailed`: warning; records a concise failure count
  and CR reference.
* `VirtualizationUnavailable`: critical only for an explicit hard capability
  absence, such as KubeVirt APIs not being served by the target.

Ordinary failed validation checks must not change the existing Provider
`Ready` result. Only `VirtualizationUnavailable` participates as a Provider
readiness blocker. No validation is created automatically in this MVP; a
user or automation creates the CR explicitly.

## VCV pull request

VCV PR #30 adds URL/token authentication. Build on that work and contribute
the generic reporting functionality upstream.

Action items:

* Make CTRF output valid and explicitly versioned (`reportFormat`,
  `specVersion`, `reportId`, `runId`, and timestamp).
* Add a snapshot reporter that publishes all selected tests as `pending` at
  run start, then records their terminal states as concurrent checks finish.
* Include stable check IDs/names, start/stop times, duration, and a bounded
  message. Do not put verbose logs or traces in snapshots.
* Serialize report generation through one reporter queue so parallel tests
  cannot generate conflicting updates.
* Add an atomic local report-file interface. A Forklift-specific wrapper may
  publish that file to Kubernetes; an optional generic ConfigMap publisher is
  acceptable if it does not couple VCV to Forklift APIs.
* Test initial, partial, terminal, timeout, skipped, and failed reports.

CTRF is suitable as a sequence of complete result snapshots because it has
`pending`, `passed`, `failed`, and `skipped` test states and run/test timing.
It does not define transport, sequencing, or optimistic concurrency; the
Forklift contract owns those concerns.

## Forklift pull request

Action items:

* Add `pkg/controller/virtualizationvalidation` and register it with the main
  controller set.
* Resolve the referenced Provider and its Secret from the hub API. Reject
  non-OpenShift Providers.
* Create a deterministic hub-local Job for the effective input hash, using a
  Forklift-owned image pinned by digest.
* Mount the Provider credential read-only. The image wrapper converts it into
  the VCV PR #30 URL/token authentication inputs without logging the token.
* Create the per-run result ConfigMap before the Job, and watch both Job and
  ConfigMap.
* Create a per-run Role/RoleBinding for the Job service account. Grant only
  `get`, `patch`, and `update` on that exact ConfigMap name; do not grant
  ConfigMap create/list/watch access to the Job.
* Parse only schema-validated, size-bounded snapshots. Ignore invalid,
  mismatched-run, stale, or regressing revisions.
* Project summary and bounded check outcomes to CR status; project the latest
  compact state to Provider conditions.
* On CR deletion or changed input hash, remove owned hub resources. Retain the
  final ConfigMap/Job long enough for diagnosis during the MVP; formal TTL and
  retention policy follow with artifacts.

## Image work

Forklift owns a derived `forklift-virt-validation` image pinned to a specific
VCV source/image revision. Do not use an unpinned `latest` image.

The wrapper must run VCV in Job mode: obtain `oc` and `virtctl`, authenticate
to the remote API with the mounted Provider credential, select only the
approved read-only `platform` catalog, and publish report snapshots. The
upstream must-gather entrypoint should not be used unchanged for this Job.

In the first controller increment, `platform` is the only accepted public
check ID (and the default). It expands to these reviewed VCV paths rather than
accepting caller-provided VCV substring filters:

* OpenShift installation, cluster version, cluster operators, node topology,
  and node health.
* OpenShift Virtualization installation.

This deliberately excludes workload-creating checks such as basic VM, live
migration, and snapshot validation. Additional IDs need an explicit review of
their VCV implementation before entering the catalog.

## Security and operational constraints

* The Provider token is mounted only into a short-lived, controller-created
  Job in the protected hub namespace. It is never present in the Job's Pod
  environment or sent to the remote target.
* Pin the VCV-derived image and base images by digest for release and
  disconnected-install mirroring.
* Limit report messages and ConfigMap data; Kubernetes ConfigMaps have a
  practical total size limit of about 1 MiB.
* Do not store raw logs, token data, remote kubeconfig content, or arbitrary
  traces in the ConfigMap or CR status.
* Rate-limit ConfigMap snapshots (for example, on state changes and no more
  often than once every one or two seconds).
* Time-bound the Job and each check; set an explicit `backoffLimit`.

## Acceptance criteria

* A user creates a Provider-profile `VirtualizationValidation` for a remote
  OpenShift Provider.
* Forklift creates owned Job, ConfigMap, Role, and RoleBinding in its namespace.
* The Job authenticates to the remote target without a pre-existing `oc login`
  and without creating any remote workload resources.
* The ConfigMap moves from an all-pending CTRF snapshot through partial
  snapshots to a valid final report.
* The CR status reflects those partial and final states.
* The Provider shows the matching compact validation condition; a normal
  failed check does not make the Provider unready.
* A missing required virtualization API produces `VirtualizationUnavailable`
  and can block Provider readiness.
* Changing `runNonce` or an effective input starts a new run and supersedes
  the prior one without exposing credentials.
* Unit tests cover input hashing, Job/Role/ConfigMap construction, report
  validation/revision handling, status projection, and cleanup. An integration
  smoke test proves CR -> Job -> partial ConfigMap -> terminal CR status.

## Follow-up increments

1. Stable result artifact retention, cancellation, and UI detail views.
2. Optional automatic Provider-profile validation and policy/defaults.
3. Plan-profile validation using a composed, Plan-specific target workload
   probe, target service account, selected storage, and explicit cleanup.
4. Plan readiness/execution gating only after the workload check and result
   contract have proven reliable.
