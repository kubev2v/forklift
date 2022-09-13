# Forklift k8s Installation

## Pre-requisites

- **Kubernetes cluster or Minikube v1.22+**
- **Operator Lifecycle Manager (OLM)**

## Installing OLM support

We strongly suggest OLM support for Forklift deployments, in some production kubernetes clusters OLM might already be present, if not, see the following examples in how to add OLM support to minikube or standard kubernetes clusters below:

### Minikube:
`$ minikube addons enable olm`

### Kubernetes:
`$ kubectl apply -f https://raw.githubusercontent.com/operator-framework/operator-lifecycle-manager/master/deploy/upstream/quickstart/crds.yaml`

`$ kubectl apply -f https://raw.githubusercontent.com/operator-framework/operator-lifecycle-manager/master/deploy/upstream/quickstart/olm.yaml`

For details and official instructions in how to add OLM support to kubernetes and customize your installation see [here](https://github.com/operator-framework/operator-lifecycle-manager/blob/master/doc/install/install.md)

### Ensure OLM health

Check OLM pods, they must all be in _Running_ state:

```
$ kubectl -n olm get pods
NAME                                READY   STATUS    RESTARTS        AGE
catalog-operator-755d759b4b-5286j   1/1     Running   1 (7d23h ago)   49d
olm-operator-c755654d4-2447p        1/1     Running   1 (7d23h ago)   49d
packageserver-d9c689b8f-57q7k       1/1     Running   1 (7d23h ago)   49d
packageserver-d9c689b8f-g6pj9       1/1     Running   1 (7d23h ago)   49d
```

## Installing _latest_

Deploy Forklift using the [forklift-k8s.yaml manifest](../forklift-k8s.yaml):

`$ kubectl apply -f https://raw.githubusercontent.com/konveyor/forklift-operator/main/forklift-k8s.yaml`

**Note**: When working with the main branch, the subscription in the manifest will pull the _latest_ operator image via development channel.

### Veryfing Operator Health

The [forklift-k8s.yaml manifest](../forklift-k8s.yaml) contains all the necesary objects to deploy operator via OLM, if successful, you should see the catalogsource, operator and OLM job pods all in _Running_ state and _Completed_ state:

```
$ kubectl -n konveyor-forklift get pods
NAME                                                  READY   STATUS      RESTARTS   AGE
d2d4595bc2822f45ea5aca8f9a09e3c65db3ee5de4574c7bceb   0/1     Completed   0          22m
forklift-operator-5489797f8c-zj6l7                    1/1     Running     0          22m
konveyor-forklift-bx8pt                               1/1     Running     0          22m
```

If this looks Ok, then you can proceed to create the forkliftcontroller CR that will install the rest of Forklift components.

### Creating a _ForkliftController_ CR (SSL/TLS disabled)
```
$ cat << EOF | kubectl -n konveyor-forklift apply -f -
apiVersion: forklift.konveyor.io/v1beta1
kind: ForkliftController
metadata:
  name: forklift-controller
  namespace: konveyor-forklift
spec:
  feature_ui: false
  feature_validation: true
  inventory_tls_enabled: false
  validation_tls_enabled: false
  must_gather_api_tls_enabled: false
  ui_tls_enabled: false
EOF
```

### Creating a _ForkliftController_ CR (SSL/TLS disabled) with UI
```
$ cat << EOF | kubectl -n konveyor-forklift apply -f -
apiVersion: forklift.konveyor.io/v1beta1
kind: ForkliftController
metadata:
  name: forklift-controller
  namespace: konveyor-forklift
spec:
  feature_ui: true
  feature_auth_required: false
  feature_validation: true
  inventory_tls_enabled: false
  validation_tls_enabled: false
  must_gather_api_tls_enabled: false
  ui_tls_enabled: false
EOF
```
