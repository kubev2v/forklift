MTV is based on upstream forklift. Its purpose is to mass migrate virtual machines from 5 source providers (VMware vSphere, oVirt, openstack, OVA files or another cluster in OpenShift) -> to their destination which is CNV / OpenShift virtualization. The provider is the original source for VMs before the migration (example VMware). The target is OpenShift. 

OpenShift has a number of operators and MTV exists as a container platform operator. 

MTV defines Kubernetes objects called Custom Resources (CR)

Before the migration starts, the person who is the OpenShift user selects the provider CR. The user would also configure a CR for network mapping and storage mapping. A network map is a CR that defines how the VM network in the provider maps to the networks in the target OpenShift cluster during migration. Similarly a storage map is a CR that defines how datastores, volumes and disks from the provider VMs should be mapped to storage in the OpenShift cluster during migration.


Two services to be aware of are the inventory service and the validation service. The inventory service (pkg/lib/inventory) gets a list of VMs from a provider as well as the virtual hardware configuration of each VM. The validation service (validation/service) makes sure the the migration is likely to be successful (example an unsupported file system may not be able to be migrated).

After the user configures the provider, network and storage CR-they can create a plan CR for the migration plan. They will choose which groups of VMs can be migrated together either grouped ("compute" directories on VMWare) or ungrouped (multiple VMs that are located anywhere on the provider) and choose which ones will have which storage and network mapping.

There are different types of migrations (cold, warm, live, conversion). All migration types support RCM or RawCopyMode. RawCopyMode shows up in the plan CR as "skipGuestConversion: true" meaning it will not use virtv2v to install virtio drivers. VDDK is required for RCM. VDDK is VMware's library that gives access to the the VM disk files or VDMK.  

  - cold migration is the default migration where the VM is turned off before the migration. This has a longer downtime. Cold migration to a remote cluster will create a VDDK DataVolume, and use CDI to transfer disks before passing them to virt-v2v. If the cold migration is to the same cluster it creates a blank DataVolume so virt-v2v can manage the data transfer itself.

 - warm migration can have shorter downtime because the VM stays on during the migration. There is one snapshot taken that is copied, then a series of snapshots where only the changes between snapshots are copied. Then finally the VM is turned off for the cutover phase and the final changes are copied-target VM is made and the guest conversion moves this copy to the target VM. This is supported for vSphere, oVirt and Red Hat virtualization.

 - Live migration has almost no downtime. This is for moving data from one cluster to another. This is only for CNV and we rely on CNV to do the migration and VirtualMachineInstanceMigration manages the state.

 - Conversion-only is not technically a migration; rather, it installs the virtIO drivers to change the guest OS and puts the changed guest OS in a target VM. Conversion-only migration needs a pre-existing Persistent Volume Claim (PVC).

Storage:

 VM disk data is copied to a PVC in OpenShift that can be used by a KubeVirt VM. In the final step of a migration, the controller creates a virtual machine CR and a VM pod/virt-launcher (logic in pkg/controller/plan) and the transferred PVCs are in the format of disks on the VM (pkg/controller/plan/adapter). 
 
  Storage maps map data stores to storage classes. 

 1 Forklift creates DataVolumes and then containerized data importer (CDI) provisions PVCs based on the DataVolumes. CDI is typically used for warm migrations but user can choose data transfer method.
 
 2 virt-v2v is part of the libguestfs project. The virt-v2v tool creates blank DataVolumes which the virt-V2V pods copy to convert the data. This is associated with cold migrations but again the user can choose. If in a cold migration the target cluster is different than the cluster where MTV is installed VDDK needs to be installed. If VDDK is not installed the migration would fail.

 3 Storage offload uses the SCSI XCOPY command to do rapid copies of VM disks directly on a supported storage array. This requires the source and destination providers to both be connected to the same storage array, and forklift must figure out how to map the source and destination storage to the correct logical unit number on the storage array so it copies the right thing.
 

 good to know where it lives: 
 
 - the konflux folder specifies dependencies needed for build and is triggered by a git commit but it should never be modified by an AI agent
 
 - the tekton folder contains the CI/CD pipeline that you see on github after you push and its triggered by a pull request and also should never be modified by an AI agent 
 
 - the build folder contains container files for the upstream and downstream of each component as well as the controller, the forklift api and forklift operator. It also has release.conf which says the mtv and ocp version.
 
 - the tests folder has end to end migration tests 
 
 - the validation folder has rego files where developers can add warnings of various levels as well as specific migration rules for vmware, OVA, openstack and oVirt.

- the operator has operator resources, manifests, configurations and the ansible playbook that forklift operator is based on.