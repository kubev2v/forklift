MTV is based on upstream forklift. Its purpose is to mass migrate data from 5 source providers (VMware vSphere, oVirt, openstack, OVA files or another cluster in openshift) -> to their destination which is CNV / openshift virtualization. The provider is where the data lives originally before the migration (example VMware). The target is openshift. 

openshift has a number of operators and MTV exists as a container platform operator. 

MTV defines kubernetes objects called Custom Resources (CR)

Before the migration starts, the person who is the openshift admin selects the provider CR. The admin would also configure a CR for network mapping and storage mapping. This means specifying that VLANs or OVN network should go to the (pod -> KubeVirt vm, multus network goes to NAD (NetworkAttachmentDefinition), ignored -> does not go anywhere its excluded from migrated vm) in openshift and the data store on the VM would go to a storage class in openshift. 

There are 2 services, an inventory service (pkg/lib/inventory) that gets the data from the vm and a validation service (validation/service) that makes sure the the data is compatible for migration (example an unsupported file system may not be able to be migrated).

After the admin configures the provider, network and storage CR-they can create a plan CR for the migration plan. They will choose which groups of providers can be migrated together and choose which ones will have which storage and mapping.

There are different types of migrations (cold, warm, live, conversion, storage). All migration types support RCM or RawCopyMode. RawCopyMode shows up in the plan CR as "skipGuestConversion: true" meaning it will not use virtv2v to install virtio drivers. VDDK is required for RCM. VDDK is VMware's library that gives access to the the VM disk files or VDMK. VDDK is also needed for warm migrations and CDI storage migrations. Migration progress is monitored in (pkg/monitoring)

  -cold migration is the default migration where the VM is turned off before the migration. This has a longer downtime. In a cold migration a DataVolume is created and virtv2v does the guest conversion by swapping proprietary drivers with its own drivers and then it goes to the target vm.

 -warm migration has shorter downtime because the VM stays on during the migration. There is one snapshot taken that is copied, then a series of snapshots where only the changes between snapshots are copied. Then finally the VM is turned off for the cutover phase and the final changes are copied-target vm is made and the guest conversion moves this copy to the target vm. This is supported for vSphere, oVirt and Red Hat virtualization.

 -Live migration has almost no downtime. This is for moving data from one cluster to another. An empty DataVolume is created in addition to a standby VM on the target cluster. Then KubeVirt migrates the storage and memory and VirtualMachineInstanceMigration manages the state.

 -Conversion only migration does NOT migrate data. It installs the virtIO drivers to change the guest OS and puts the changed guest OS in a target vm.

 -storage migration copies VM disk data to a persistent volume claim PVC in openshift that can be used by a KubeVirt VM. In the final step of the migration, the controller creates a virtual machine CR and a vm pod/virt-launcher and the transferred PVCs are in the format of disks on the VM (pkg/controller/plan/adapter). These are the formats needed for for a storage migration for each provider: 
 
 vSphere uses CDI with VDDK, oVirt/Red Hat Virtualization uses CDI with an ImageIO, KubeVirt in OpenShift uses an export API. 
 
 The storage configuration options are as follows: volume modes (file system or block), Access Modes (ReadWriteOnce or ReadWriteMany), Storage Classes defines performance and maps data stores to storage classes. 

 1 containerized data importer or CDI migration is the default. CDI creates DataVolumes that provision a persistent volume claim PVC that is an abstraction of a DataVolume. (logic in cmd/*-populator)
 
 2 virt-v2v migration uses the the libguestfs virt-v2v tool. This migration creates blank DataVolumes which the virt-V2V pods copy to convert the data.

 3 Storage offload migration uses storage arrays in XCOPY to copy data between LUN or Logical Unit Number to identify the storage volume. 

 The cold migration workflow for storage is: The target DataVolumes are created-CDI makes PVCs based on that and starts an import pod. (logic in pkg/controller/plan) The import pod copies the data and if using option 2 above ^^ the virt-v2v conversion pod runs (virt-v2v tool is upstream from forklift). Finally the populated PVC will appear in the target VM. 
 
 The warm mmigration storage workflow is: see warm migration workflow above ^^

 good to know where it lives: the konflux folder builds container images and is triggered by a git commit, the tekton folder contains the CI/CD pipeline that you see on github after you push and its triggered by a pull request, the build folder contains docker files for each component as well as the controller, the forklift api and forklift operator, the tests folder has end to end migration tests and the validation folder has rego files where developers can add warnings of various levels as well as specific migration rules for vmware, OVA, openstack and oVirt.