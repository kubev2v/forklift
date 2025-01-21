![Build](https://github.com/kubev2v/forklift/workflows/Build%20and%20push%20images/badge.svg)&nbsp;![CI](https://github.com/kubev2v/forklift/workflows/CI/badge.svg)&nbsp;[![Code Coverage](https://codecov.io/gh/kubev2v/forklift/branch/main/graph/badge.svg?token=VV6EBWKJGB)](https://codecov.io/gh/kubev2v/forklift)

# Forklift

Forklift controller.

---

## Deploy
Deploy the latest Forklift operator index to the cluster
```bash
make deploy-operator-index REGISTRY_TAG=latest
```


## Build
Custom build of the controller, bundle and index which will be deployed to the cluster
```bash
export REGISTRY_ORG=user
make push-controller-image \
     push-operator-bundle-image \
     push-operator-index-image \
     deploy-operator-index
```
Note: The order of targets is important as the bundle needs to be created after controller and index after bundle.

### Configuration

| Name                       | Default value                                  | Description                                                            |
|----------------------------|------------------------------------------------|------------------------------------------------------------------------|
| REGISTRY_TAG               | devel                                          | The tag with which the image will be built and pushed to the registry. |
| REGISTRY_ORG               | kubev2v                                        | The registry organization to which the built image should be pushed.   |
| REGISTRY                   | quay.io                                        | The registry address to which the images should be pushed.             |
| CONTAINER_CMD              | autodetected                                   | The container runtime command (e.g.: /usr/bin/podman)                  |
| VERSION                    | 99.0.0                                         | The version with which the forklift should be built.                   |
| NAMESPACE                  | konveyor-forklift                              | The namespace in which the operator should be installed.               |
| CHANNELS                   | development                                    | The olm channels.                                                      |
| DEFAULT_CHANNEL            | development                                    | The default olm channel.                                               |
| OPERATOR_IMAGE             | quay.io/kubev2v/forklift-operator:latest       | The forklift operator image with the ansible-operator role.            |
| CONTROLLER_IMAGE           | quay.io/kubev2v/forklift-controller:latest     | The forklift controller image.                                         |
| MUST_GATHER_IMAGE          | quay.io/kubev2v/forklift-must-gather:latest    | The forklift must gather an image.                                     |
| UI_PLUGIN_IMAGE            | quay.io/kubev2v/forklift-console-plugin:latest | The forklift OKD/OpenShift UI plugin image.                            |
| VALIDATION_IMAGE           | quay.io/kubev2v/forklift-validation:latest     | The forklift validation image.                                         |
| VIRT_V2V_IMAGE             | quay.io/kubev2v/forklift-virt-v2v:latest       | The forklift virt v2v image for cold migration.                        |
| POPULATOR_CONTROLLER_IMAGE | quay.io/kubev2v/populator-controller:latest    | The forklift volume-populator controller image.                        |
| OVIRT_POPULATOR_IMAGE      | quay.io/kubev2v/ovirt-populator:latest         | The oVirt populator image.                                             |
