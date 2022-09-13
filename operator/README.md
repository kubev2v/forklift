# Forklift Operator

[![Operator Repository on Quay](https://quay.io/repository/konveyor/forklift-operator/status "Operator Repository on Quay")](https://quay.io/repository/konveyor/forklift-operator) [![License](http://img.shields.io/:license-apache-blue.svg)](http://www.apache.org/licenses/LICENSE-2.0.html) [![contributions welcome](https://img.shields.io/badge/contributions-welcome-brightgreen.svg?style=flat)](https://github.com/konveyor/forklift-operator/pulls)

Forklift Operator installs a suite of migration tools that facilitate the migration of VM workloads to [OpenShift Virtualization](https://cloud.redhat.com/learn/topics/virtualization/) or [KubeVirt](https://kubevirt.io/).

## Prerequisites

* [__OpenShift 4.9+__](https://www.openshift.com/) or [__k8s v1.22+__](https://kubernetes.io/)
* [__OpenShift Virtualization 4.9+__](https://www.redhat.com/en/technologies/cloud-computing/openshift/) or [__KubeVirt__](https://kubevirt.io/)
* [__Operator Lifecycle Manager (OLM) support__](https://olm.operatorframework.io/)

## Compatibility

OpenShift Virtualization/KubeVirt is required and must be installed prior attempting to deploy Forklift, see the table below for supported configurations:

Forklift release | OpenShift Virtualization/KubeVirt | VMware | oVirt
--- | --- | --- | --- 
v2.2 | v4.9 | 6.5+ | 4.4.9+
v2.3 | v4.10+ | 6.5+ | 4.4.9+

**Note:** Please keep in mind Forklift will not deploy in unsupported configurations.

## Component Overview

The operator will install all the necessary components which Forklift needs to operate. The projects and a description of each are detailed below:

* [Forklift UI](https://github.com/konveyor/forklift-ui), The Forklift UI is based on [Patternfly 4](https://www.patternfly.org/v4).
* [Forklift Controller](https://github.com/konveyor/forklift-controller), The Forklift Controller orchestrates the migration.
* [Forklift Validation](https://github.com/konveyor/forklift-validation), The Forklift Validation service checks the VMs for possible issues before migration. This service is based on [Open Policy Agent](https://www.openpolicyagent.org).
* [Forklift Must Gather](https://github.com/konveyor/forklift-must-gather), Support tool for gathering information about the Forklift environment.

## Development

See [development.md](docs/development.md) for details in how to contribute to Forklift operator.

## Forklift Operator Installation on OKD/OpenShift

The method used for these instructions relies on OKD/OCP Web Console, it is also possible to automate the deployment in OpenShift using manifests if needed, please check the [k8s deployment manifest](./forklift-k8s.yaml) for details.

### Installing _released versions_

Released (or public betas) of Forklift are installable via community operators which appear in [OCP](https://openshift.com/) and [OKD](https://www.okd.io/) marketplace.

1. Visit the OpenShift Web Console.
1. Navigate to _Operators => OperatorHub_.
1. Search for _Forklift Operator_.
1. Install the desired _Forklift Operator_ version.

### Installing _latest_ (or other unreleased versions)

Installing latest is almost an identical procedure to released versions but requires creating a new catalog source.

1. `oc create -f forklift-operator-catalog.yaml`
1. Follow the same procedure as released versions until the Search for _Forklift Operator_ step.
1. There should be two _Forklift Operator_ available for installation now.
1. Select the _Forklift Operator_ without the _community_ tag.
1. Proceed to install latest using the _development_ channel in the subscription step.

**Note:** Installing _latest_ may also include OLM channels for other released versions.

### ForkliftController CR Creation

Once you have successfully installed the operator, proceed to deploy components by creating the _ForkliftController_ CR.

1. Visit OpenShift Web Console, navigate to _Operators => Installed Operators_.
1. Select _Forklift Operator_.
1. Locate _ForkliftController_ on the top menu and click on it.
1. Adjust settings if desired and click Create instance.

Once the CR is created, the operator will deploy the controller, UI and configure the rest of required components.

## Installing on Kubernetes (or Minikube)

See [k8s.md](./docs/k8s.md) for details.

## Customize Settings

Custom deployment settings can be applied by editing the `ForkliftController` CR.

`oc edit forkliftcontroller -n konveyor-forklift`

## Removing Forklift Operator

Use the [Forklift cleanup script](./tools/forklift-cleanup.sh), this is the recommended method to delete operator, CRDs and all related objects. It supports OpenShift and Kubernetes environments.

`forklift-cleanup.sh -o`

## Forklift Documentation

See the [Forklift Documentation](https://konveyor.github.io/forklift/) for detailed installation instructions as well as how to use Forklift.
