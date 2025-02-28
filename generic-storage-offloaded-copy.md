prerequisite:
-  storage class which support it must have the annotation "copy-offload"
   those storage classes are known to be shared with vmware and OCP and support XCOPY
modified flow:
- Start a migration steps from OCP
  migration.go -> run
      migration.go -> begin
      migration.go -> execute
- OCP CSI creates a PVC, LUN is created on storage (no change, regular flow)
    migration.go -> phase 
  - sets an annotation on PVC "copy-offload" if the destination storage class has "copy-offload" annotation
  - set DataVolume source to blank image like cold copy
  
- New pre-copy-phase based on annotation on storage class annotation is waiting for an annotation
  on the migration resource
- Storage admin unmap from OCP and maps the LUN to ESX
- Migration admin sets the annotation on the migration resource mapped-to-vmware
- New copy-phase spins a pod to perform vmkfstools on ESXi using SSH . Wait for annotation mapped-to-ocp
  to complete
- Storage admin unmaps from VMware and maps the LUN to OCP
- Migration admin sets the annotation on the migration resource mapped-to-ocp
- Next migration phase creates PersistentVolume and PVC from the LUN details for virt-v2v pod
- Next phase performs virt-v2v-in-place on PV (as block device)


