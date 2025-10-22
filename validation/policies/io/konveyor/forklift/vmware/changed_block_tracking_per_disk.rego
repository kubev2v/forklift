package io.konveyor.forklift.vmware
import future.keywords.in

change_tracking_disabled_per_disk(disk) {
    disk.changeTrackingEnabled == false
}

concerns[flag] {
    some disk in input.disks
    change_tracking_disabled_per_disk(disk)

    path_parts := split(disk.file, "/")
    filename := trim(path_parts[count(path_parts) - 1], "] ")

    baseKey := (disk.controllerKey / 100) * 100
    controllerIndex := disk.controllerKey - baseKey

    deviceKey := sprintf("%s%d:%d", [disk.bus, controllerIndex, disk.unitNumber])

    flag := {
        "id": "vmware.changed_block_tracking.disk.disabled",
        "category": "Warning",
        "label": sprintf("Disk - %s does not have CBT enabled", [deviceKey]),
        "assessment": "Changed Block Tracking (CBT) has not been enabled for this device. This feature is a prerequisite for VM warm migration."
    }
}
