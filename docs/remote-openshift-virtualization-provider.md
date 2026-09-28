# Remote OpenShift Virtualization Provider setup

Use this guide to add a remote OpenShift Virtualization cluster as a Forklift
OpenShift Provider and to enable its optional validation workflow.

## Trust model

The remote ServiceAccount token and API CA are stored in the Provider Secret
on the hub cluster. Forklift mounts that Secret only into a short-lived,
hub-local validation Job; it is never written to validation status.

Validation uses the selected Provider credential. It does not currently accept
a separate ServiceAccount for an individual validation run. Use a dedicated
remote ServiceAccount for the Provider if this access must be isolated.

## Remote ServiceAccount and RBAC

Run these manifests on the remote cluster. The ServiceAccount namespace is an
administrative namespace; it is separate from the namespace used for test VMs.

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: mtv-validation
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: forklift-validation
  namespace: mtv-validation
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: forklift-validation-platform
rules:
  # Provider connection and platform validation.
  - apiGroups: [""]
    resources: ["namespaces", "nodes"]
    verbs: ["get", "list"]
  - apiGroups: ["config.openshift.io"]
    resources: ["clusterversions", "clusteroperators"]
    verbs: ["get", "list"]
  - apiGroups: ["kubevirt.io"]
    resources: ["kubevirts"]
    verbs: ["get", "list"]
  - apiGroups: ["cdi.kubevirt.io"]
    resources: ["datasources"]
    verbs: ["get", "list"]
  - apiGroups: ["k8s.cni.cncf.io"]
    resources: ["network-attachment-definitions"]
    verbs: ["get", "list"]
  # Discovery of the OpenShift Virtualization virtctl download route.
  - apiGroups: ["route.openshift.io"]
    resources: ["routes"]
    verbs: ["get", "list"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: forklift-validation-platform
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: forklift-validation-platform
subjects:
  - kind: ServiceAccount
    name: forklift-validation
    namespace: mtv-validation
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: forklift-validation-virtctl-download
  namespace: openshift-cnv
rules:
  - apiGroups: [""]
    resources: ["endpoints"]
    verbs: ["get"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: forklift-validation-virtctl-download
  namespace: openshift-cnv
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: forklift-validation-virtctl-download
subjects:
  - kind: ServiceAccount
    name: forklift-validation
    namespace: mtv-validation
```

A Provider used for migration can need additional inventory and migration
permissions. This PoC validation RBAC is not a replacement for the supported
MTV migration credential requirement: the product documentation requires an
OpenShift Virtualization ServiceAccount token with `cluster-admin` privileges
when adding a Provider. Review this manifest against the target-cluster policy
before using it outside the validation PoC.

## Dedicated workload-validation namespace

Workload validation creates a VM, VMI, DataVolume, and PVC, waits for the VM,
then removes everything it created. Never use a namespace containing user
workloads.

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: mtv-validation-workloads
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: forklift-validation-workload
  namespace: mtv-validation-workloads
rules:
  - apiGroups: ["kubevirt.io"]
    resources: ["virtualmachines", "virtualmachineinstances"]
    verbs: ["get", "list", "watch", "create", "delete"]
  - apiGroups: ["cdi.kubevirt.io"]
    resources: ["datavolumes"]
    verbs: ["get", "list", "watch", "delete"]
  - apiGroups: [""]
    resources: ["persistentvolumeclaims"]
    verbs: ["get", "list", "watch", "delete"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: forklift-validation-workload
  namespace: mtv-validation-workloads
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: forklift-validation-workload
subjects:
  - kind: ServiceAccount
    name: forklift-validation
    namespace: mtv-validation
```

The current basic workload check uses the `rhel10` DataSource and falls back
to `rhel9` in `openshift-virtualization-os-images`. Ensure that one of those
DataSources has a usable backing PVC. Test clusters exposing only versioned
DataSources need a usable alias or a VCV image configured for that source.

## Provider Secret and Provider

On the remote cluster, create a bounded token and obtain the service CA:

```bash
REMOTE_TOKEN="$(oc -n mtv-validation create token forklift-validation --duration=24h)"
oc -n mtv-validation get configmap kube-root-ca.crt \
  -o jsonpath='{.data.ca\.crt}' > remote-ca.crt
```

On the hub cluster, create the Provider Secret and Provider. Replace
`konveyor-forklift` with the Forklift namespace and set the remote API URL.

```bash
oc -n konveyor-forklift create secret generic remote-cnv-credentials \
  --from-literal=token="${REMOTE_TOKEN}" \
  --from-file=ca.crt=remote-ca.crt
unset REMOTE_TOKEN
```

```yaml
apiVersion: forklift.konveyor.io/v1beta1
kind: Provider
metadata:
  name: remote-cnv
  namespace: konveyor-forklift
spec:
  type: openshift
  url: https://api.remote.example.com:6443
  secret:
    name: remote-cnv-credentials
    namespace: konveyor-forklift
```

Use the remote API CA whenever possible. `insecureSkipVerify: "true"` is only
a temporary diagnostic option and must not be used together with `ca.crt`.

Verify the connection:

```bash
oc -n konveyor-forklift get provider remote-cnv -w
```

## Validation modes

The Provider **Validation** tab offers:

* **Platform validation**: remote API and OpenShift Virtualization readiness.
* **Workload validation**: platform validation plus VM create, start, and
  cleanup in the dedicated remote namespace.
* **Demo: simulated failure**: the workload sequence plus a deterministic,
  non-mutating failed check. This is PoC-only and must not be in a production
  validator image.

The hub-local `VirtualizationValidation` CR receives progressive CTRF results.
Its status is the user-facing detailed result surface.

## Token rotation

`oc create token` returns an expiring token. Before expiration, create a new
token and replace the Provider Secret's `token` value. Later validation runs
use the new Secret resource version. Do not use a long-lived token merely to
avoid this rotation step.
