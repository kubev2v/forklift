# ovirt-volume-populator
 
[Volume Populator](https://kubernetes.io/blog/2022/05/16/volume-populators-beta/) that uses disks from oVirt as a source.
Currently only tested on Openshift

## Installation

```shell
$ git clone https://github.com/bennyz/ovirt-imageio-populator
$ cd ovirt-imageio-populator
$ oc apply -f https://raw.githubusercontent.com/kubernetes-csi/volume-data-source-validator/v1.3.0/client/config/crd/populator.storage.k8s.io_volumepopulators.yaml
$ oc apply -f https://raw.githubusercontent.com/kubernetes-csi/volume-data-source-validator/v1.3.0/deploy/kubernetes/rbac-data-source-validator.yaml
$ oc apply -f https://raw.githubusercontent.com/kubernetes-csi/volume-data-source-validator/v1.3.0/deploy/kubernetes/setup-data-source-validator.yaml

# Openshift + kuberenetes < 1.24:
$ oc edit featuregate cluster
# Add:
# spec:
#  customNoUpgrade:
#    enabled:
#    - AnyVolumeDataSource
#  featureSet: CustomNoUpgrade
$ oc apply -f example/crd.yaml
$ oc apply -f example/deploy.yaml
```
