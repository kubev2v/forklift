# vsphere-xcopy-volume-populator

This volume populator implementation is specific for performing XCOPY from a source vmdk 
disk file, to a target PVC.
The way it works is by performing the XCOPY using vmkfstools on the target ESXi.

Limitations:
- The source VMDK must sit on a LUN from the same storage array endpoint where the target LUN
would be created.
- Progress reporting is missing because of lack of underlying tooling support (vmkfstools)

# controller
The controller uses the standard volume populator library from kubernetes, and is 
compiled to bin/manager binary, with the responsibility to schedule the popoulator pod
with the right command line arguments.

# populator
The populator, invoked with --mode=populate, is responsible for the copy process itself, and the update
of the progress on the pvc

# vmkfstools-wrapper
Scripts to create a VIB to wrap the vmkfstools as an ESXCli extension.
The VIB should be installed on every ESXi that is connected to the datastores which
are holds migratable VMs.
See vmkfstools-wrapper/README.md
