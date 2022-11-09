![CI](https://github.com/konveyor/forklift-controller/workflows/CI/badge.svg)&nbsp;[![Code Coverage](https://codecov.io/gh/konveyor/forklift-controller/branch/master/graph/badge.svg)](https://codecov.io/gh/konveyor/forklift-controller)

# forklift-controller

Konveyor Forklift controller.

---

## Build

For the build, the forklift uses [Bazel](https://bazel.build/).

### Configuration

The environment which you can set across all projects.

| Name             | Default value | Description                                                            |
|------------------|---------------|------------------------------------------------------------------------|
| REGISTRY_TAG     | devel         | The tag with which the image will be built and pushed to the registry. |
| REGISTRY_ACCOUNT |               | The user account name to which the built image should be pushed.       |
| REGISTRY         | quay.io       | The registry address to which the images should be pushed.             |

## Operator

### Variables

The environment variables that you can set in .bazelrc, these variables are used during Bazel build process and used inside the bazel sandbox.
Another option to override the default values can use `--action_env` as in the example.

| Name                  | Default value                                    | Description                                                 |
|-----------------------|--------------------------------------------------|-------------------------------------------------------------|
| VERSION               | 99.0.0                                           | The version with which the forklift should be built.        |
| NAMESPACE             | konveyor-forklift                                | The namespace in which the operator should be installed.    |
| CHANNELS              | development                                      | The olm channels.                                           |
| DEFAULT_CHANNEL       | development                                      | The default olm channel.                                    |
| OPERATOR_IMAGE        | quay.io/konveyor/forklift-operator:latest        | The forklift operator image with the ansible-operator role. |
| CONTROLLER_IMAGE      | quay.io/konveyor/forklift-controller:latest      | The forklift controller image.                              |
| MUST_GATHER_IMAGE     | quay.io/konveyor/forklift-must-gather:latest     | The forklift must gather an image.                          |
| MUST_GATHER_API_IMAGE | quay.io/konveyor/forklift-must-gather-api:latest | The forklift must gather image api.                         |
| UI_IMAGE              | quay.io/konveyor/forklift-ui:latest              | The forklift UI image.                                      |
| UI_PLUGIN_IMAGE       | quay.io/kubev2v/forklift-console-plugin:latest   | The forklift OKD/OpenShift UI plugin image.                                      |
| VALIDATION_IMAGE      | quay.io/konveyor/forklift-validation:latest      | The forklift validation image.                              |
| VIRT_V2V_IMAGE        | quay.io/konveyor/forklift-virt-v2v:latest        | The forklift virt v2v image.                                |

### Runing operator build

```bash
export REGISTRY_ACCOUNT=username
export REGISTRY=quay.io
export REGISTRY_TAG=latest

CONTROLLER_IMAGE=quay.io/${REGISTRY_ACCOUNT}/forklift-controller:${REGISTRY_TAG}
OPERATOR_IMAGE=quay.io/${REGISTRY_ACCOUNT}/forklift-operator:${REGISTRY_TAG}

bazel run push-forklift-operator
bazel run push-forklift-operator-bundle --action_env OPERATOR_IMAGE=${OPERATOR_IMAGE} --action_env CONTROLLER_IMAGE=${CONTROLLER_IMAGE}
# The build of the catalog requires already pushed bundle
# For http registry add --action_env OPM_OPTS="--use-http"
bazel run push-forklift-operator-index --action_env REGISTRY=${REGISTRY} --action_env REGISTRY_ACCOUNT=${REGISTRY_ACCOUNT} --action_env REGISTRY_TAG=${REGISTRY_TAG}
```

---

## Logging

Logging can be configured using environment variables:

- LOG_DEVELOPMENT: Development mode with human readable logs
  and (default) verbosity=4.
- LOG_LEVEL: Set the verbosity.

Verbosity:

- Info(0) used for `Info` logging.
  - Reconcile begin,end,error.
  - Condition added,update,deleted.
  - Plan postponed.
  - Migration (k8s) resources created,deleted.
  - Migration started,stopped,run (with phase),canceled,succeeded,failed.
  - Snapshot created,updated,deleted,changed.
  - Inventory watch ensured.
  - Policy agent disabled.
- Info(1) used for `Info+` logging.
  - Connection testing.
  - Plan postpone detials.
  - Pending migration details.
  - Migration (k8s) resources found,updated.
  - Scheduler details.
- Info(2) used for `Info++` logging.
  - Full conditions list.
  - Migrating VM status (full definition).
  - Provider inventory data reconciler started,stopped.
- Info(3) used for `Info+++` logging.
  - Inventory watch: resources changed;queued reconcile events.
  - Data reconciler: models created,updated,deleted.
  - VM validation succeeded.
- Info(4) used for `Debug` logging.
  - Policy agent HTTP request.

---

## Profiler

The profiler can be enabled using the following environment variables:

- PROFILE_KIND: Kind of profile (memory|cpu|mutex).
- PROFILE_PATH: Profiler output directory.
- PROFILE_DURATION: The duration (minutes) the profiler
  will collect data. (0=indefinately)
