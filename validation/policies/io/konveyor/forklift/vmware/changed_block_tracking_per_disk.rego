package io.konveyor.forklift.vmware
import future.keywords.in

change_tracking_disabled_per_disk(disk) {
    disk.changeTrackingEnabled == false
}

concerns[flag] {
    some disk in input.disks  # Iterate over disks
    change_tracking_disabled_per_disk(disk)
    path_parts := split(disk.file, "/")
    filename := trim(path_parts[count(path_parts) - 1], "] ")
    
    flag := {
        "category": "Warning",
        "label": sprintf("Disk (key: %d, file: %s, datastore: %s) does not have CBT enabled",[disk.key, filename, disk.datastore.id]),
        "assessment": "Changed Block Tracking (CBT) has not been enabled on this VM. This feature is a prerequisite for VM warm migration."
    }
}
