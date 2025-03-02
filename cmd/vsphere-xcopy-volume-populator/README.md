# vsphere-xcopy-volume-populator

This volume populator implementation is specific for performing XCOPY from a source vmdk 
disk file, to a target PVC.
The way it works is by performing the XCOPY using vmkfstools on the target ESXi.

Limitations:
- The source VMDK must sit on a LUN from the same storage array endpoint where the target LUN
would be created.
- Progress reporting is missing because of lack of underlying tooling support (vmkfstools)

## Matching PVC with DataStores to deduce copy-offload support:
For XCOPY to be supported a source VMDK disk backing LUN (iSCSI or FC) must co exist
with the target PVC (backed by a lun) on the same storage array.
When a user is picking a VM to migrate to Openshift there is no direct indication
of that info, other then if the current storage mapping supports it or not.
The plan should know if a specific disk should use the xcopy populator by a flag 
'useCopyOffload' on the storage mapping, if technically it meets the conditions
stated above.
To detect those conditions this heuristics is used:
- locate the LUN where the vmdk disk is on iSCSI or FC
- the PVC CSI provisioner creates LUNs on the same system as the VMFS where vmdks are

An example ConfigMap for the mapping:
```
apiVersion: v1
kind: ConfigMap
metadata:
  name: copy-offload-mapping
  namespace: openshift-mtv
data:
    # name of the storage class
    storageClassMapping:
        storage-class-abc:
            storageProductVendor: ontap
            # name of the vsphere provider configured in forklift
            vsphere-provider-id-abc:
                # Vsphere data-store
                - ds-iscsi-1
                - ds-iscsi-2
            vsphere-provider-id-def:
                - ds-iscsi-3
        storage-class-lmn:
            storageProductVendor: powerStore
            vsphere-provider-id-def:
                - ds-iscsi-5
                - ds-iscsi-6
```

According to this ConfigMap a migration plan to for 'vm-5' with storage mapping
of 'ds-iscsi-3' to storageClass 'storage-class-lmn' will use the populator with
storage product vendor 'powerStore'.

# controller
The controller uses the forked volume populator library from kubernetes in
forklift, and is compiled to bin/manager binary, with the responsibility to
schedule the populator pod with the right command line arguments.

# populator
The populator is responsible for the copy process itself, and the update
of the progress on the PVC.

# vmkfstools-wrapper
Scripts to create a VIB to wrap the vmkfstools as an ESXCli extension.
The VIB should be installed on every ESXi that is connected to the datastores which
are holds migratable VMs.
See vmkfstools-wrapper/README.md
