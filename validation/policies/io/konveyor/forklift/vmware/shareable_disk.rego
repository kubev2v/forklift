package io.konveyor.forklift.vmware

has_shareable_disk {
    some i
    input.disks[i].shared
}

concerns[flag] {
    has_shareable_disk
    flag := {
        "category": "Warning",
        "label": "Shareable disk detected",
        "assessment": "Shared disks are only supported by certain OpenShift Virtualization storage configurations. Ensure that the correct storage is selected for the disk."
    }
}
