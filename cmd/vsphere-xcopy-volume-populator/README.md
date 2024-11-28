# vsphere-xcopy-volume-populator


## Forklift Controller
When the feature flag `feature_copy_offload` is true (off by default), the controller will
consult a config map to decided if VM disk from VMWare could be copied
by the storage backend(offloaded) into the newly created PVC.
When the controller creates the PVC for the v2v pod it will also create
a volume popoulator resource of type VSphereXcopyVolumePopulator and set
the filed `dataSourceRef` in the PVC to reference it.

## Populator Controller
Added a new populator controller for the resource VSPhereXcopyVolumePopulator

## VSphereXcopyVolumePopulator
A new populator implementation under cmd/vsphere-xcopy-volume-populator
is a cli program that runs in a container that is responsible to perform
XCOPY to effciently copy data from a VMDK to the target PVC. See the
flow chart below.
The populator uses the storage API (configurable) to map the PVC to an ESX 
then uses Vsphere API to call functions on the ESX to perform the actual
XCOPY command (provided that VAAI and accelerations is enabled on that
ESX).

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


## Matching PVC with DataStores to deduce copy-offload support:
For XCOPY to be supported a source VMDK disk backing LUN (iSCSI or FC) must co exist
with the target PVC (backed by a LUN) on the same storage array.
When a user is picking a VM to migrate to OpenShift there is no direct indication
of that info, other then if the current storage mapping supports it or not.
The plan should know if a specific disk should use the XCOPY populator by matching
the source vmdk data-store with the storage class. The supported pair of such
mapping is specified in a config map name 'copy-offload-mapping' along with 
other storage specific identifiers.

To detect those conditions this heuristics is used:
- locate the LUN where the vmdk disk is on iSCSI or FC
- the PVC CSI provisioner creates LUNs on the same system as the VMFS where vmdks are

An example ConfigMap for the mapping:
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: copy-offload-mapping
  namespace: openshift-mtv
data:
    # name of the storage class
    storageClassMapping: |
        storage-class-1:
            storageVendorProduct: productX
            # secret with the specific storage vendor under the forklift controller namespace
            storageVendorSecretRef: ontap-1
            vsphereProviders:
              - name: vsphere-provider-id-1
                dataStores:
                  - ds-iscsi-1
                  - ds-iscsi-2
              - name: vsphere-provider-id-2
                dataStores:
                  - ds-iscsi-3

        storage-class-2:
            storageVendorProduct: productY
            storageVendorSecretRef: ontap-2
            vsphere-provider-id-def:
                - ds-iscsi-5
                - ds-iscsi-6
```

According to this ConfigMap a migration plan to for 'vm-5' with storage mapping
of 'ds-iscsi-3' to storageClass 'storage-class-2' will use the populator with
storage product vendor 'productY'.



