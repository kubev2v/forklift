# vSphere XCOPY Volume-Populator

## Forklift Controller
When the feature flag `feature_copy_offload` is true (off by default), the controller
consult the storagemaps offload plugin configuration, to decided if VM disk from
VMWare could be copied by the storage backend(offloaded) into the newly created PVC.
When the controller creates the PVC for the v2v pod it will also create
a volume popoulator resource of type VSphereXcopyVolumePopulator and set
the filed `dataSourceRef` in the PVC to reference it.

## Populator Controller
Added a new populator controller for the resource VSPhereXcopyVolumePopulator

## VSphereXcopyVolumePopulator Resource
A new populator implementation under cmd/vsphere-xcopy-volume-populator
is a cli program that runs in a container that is responsible to perform
XCOPY to effciently copy data from a VMDK to the target PVC. See the
flow chart below.
The populator uses the storage API (configurable) to map the PVC to an ESX 
then uses Vsphere API to call functions on the ESX to perform the actual
XCOPY command (provided that VAAI and accelerations is enabled on that
ESX).

Example of the new resource and a PVC referencing it:
```

apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: my-pvc
  namespace: default
spec:
  resources:
    requests:
      storage: 100000Mi
  dataSourceRef:
    apiGroup: forklift.konveyor.io
    kind: VSphereXcopyVolumePopulator
    name: vm-1-xcopy-1
  storageClassName: sc-1  
  volumeMode: Block
  volumeName: pvc-6dff02f2-de63-40ab-a534-3bd5a7b47f82
---
apiVersion: forklift.konveyor.io/v1beta1
kind: VSphereXcopyVolumePopulator
metadata:
  name: vm-1-xcopy-1 
  namespace: default
spec:
  secretRef: vantara-secret 
  storageVendorProduct: vantara
  targetPVC: my-pvc 
  vmdkPath: '[my-vsphere-ds] vm-1/vm-1.vmdk'
```

## vmkfstools-wrapper
An ESXi CLI extension that exposes the vmkfstools clone operation to API interaction.
The folder vmkfstools-wrapper has a script to create a VIB to wrap the vmkfstools-wrapper.sh
to be a proxy perform vmkfstools commands and more.
The VIB should be installed on every ESXi that is connected to the datastores which
are holds migratable VMs.
See vmkfstools-wrapper/README.md for the installation of the tool using ansible

## Storage Provider
If a storage provider wants to their storage to be supported they need
to implement a go package named after their product, and mutate main so
their specific code path is initialized. See
cmd/vsphere-xcopy-volume-populator/internal/populator/storage.go

# Limitation
- currently a VM with single disk is supported
- The source VMDK must sit on a LUN from the same storage array endpoint where
the target LUN would be created.
- Progress reporting is missing because of lack of underlying tooling support (vmkfstools)

This volume populator implementation is specific for performing XCOPY from a source vmdk 
disk file, to a target PVC.
The way it works is by performing the XCOPY using vmkfstools on the target ESXi.


<a id="matching-pvc"></a>
## Matching PVC with DataStores to deduce copy-offload support
For XCOPY to be supported a source VMDK disk backing LUN (iSCSI or FC) must co exist
with the target PVC (backed by a LUN) on the same storage array.
When a user is picking a VM to migrate to OpenShift there is no direct indication
of that info, other then if the current storage mapping supports it or not.
The plan should know if a specific disk should use the XCOPY populator by matching
the source vmdk data-store with the storage class. The supported pair of such
mapping is specified in the migration plan storagemap object.

To detect those conditions this heuristics is used:
- locate the LUN where the vmdk disk is on iSCSI or FC
- the PVC CSI provisioner creates LUNs on the same system as the VMFS where vmdks are

An example `StorageMap` for copy offload: 
```yaml
apiVersion: forklift.konveyor.io/v1beta1
kind: StorageMap
metadata:
  name: copy-offload
  namespace: openshift-mtv
spec:
  map:
  - destination:
      storageClass: YOUR_STORAGE_CLASS  #1)
    offloadPlugin:
      vsphereXcopyConfig:
        secretRef: SECRET_WITH_ONTAP_CREDS #2)
        storageVendorProduct: ontap #3)
    source:
      id: DATASTORE_ID #4) eg datastore-18601
  provider:
    destination:
      apiVersion: forklift.konveyor.io/v1beta1
      kind: Provider
      name: host
      namespace: openshift-mtv
      uid: YOUR_HOST_PROVIDER_ID #5)
    source:
      apiVersion: forklift.konveyor.io/v1beta1
      kind: Provider
      name: YOUR_VSPHERE_PROVIDER_NAME #6)
      namespace: openshift-mtv
      uid: YOUR_VSPHERE_PROVIDER_ID  #7)

```

1. the storage class for the target PVC of the VM
2. secret with the storage provider credentials 
3. string that identifies the storage product.
4. datastore ID as set by vSphere 
5. host provider ID
6. vsphere provider name
7. vsphere provider id

# Secret with storage provider credentials

## Hitachi Vantara
- see [README](internal/vantara/README.md)

# Setup copy offload
- Set the feature flag
  `oc patch forkliftcontrollers.forklift.konveyor.io forklift-controller --type merge -p '{"spec": {"feature_copy_offload": "true"}}' -n openshift-mtv`
- Set the volume-populator image (should be unnecessary in 2.8.5)
  `oc set env -n openshift-mtv deployment forklift-volume-populator-controller --all VSPHERE_XCOPY_VOLUME_POPULATOR_IMAGE=quay.io/kubev2v/vsphere-xcopy-volume-populator`
- Create a `StorageMap` according to [this section](#matching-pvc)
- Create a plan and make sure to edit the mapping section and set the name to the `StorageMap` previously created
  Here is how the mapping part looks in a `Plan`:
  ```yaml

    apiVersion: forklift.konveyor.io/v1beta1
    kind: Plan
    metadata:
      name: my-plan
    spec:
      map:
        storage:
          apiVersion: forklift.konveyor.io/v1beta1
          kind: StorageMap
          name: copy-offload  # <-- This points to the StorageMap configured previously
          namespace: openshift-mtv
  ```

# Troubleshooting

## vSphere/ESXi
- Sometimes remote ESXi execution can fail with SOAP error with no apparent root cause message
  Since VSphere is invoking some SOAP/Rest endpoints on the ESXi, those can fail because of 
  standard error reasons and vanish after the next try. If the popoulator fails the migration
  can be restarted. We may want to restart/retry that populator or restart the migration.

## NetApp
- Error `cannot derive SVM to use; please specify SVM in config file`
  This is a configuration issue with Ontap and could be fixed by specifying a default
  SVM using vserver commands on the ontap server:
  ```
  # show current config for an SVM
  vserver show -vserver ${NAME_OF_SVM}
  ...
  ```
  Try to set a mgmt interface for the SVM and put that hostname in the STORAGE_HOSTNAME

