package io.konveyor.forklift.vmware

has_rdm_disk {
    some i
    input.disks[i].rdm
}

concerns[flag] {
    has_rdm_disk
    flag := {
        "category": "Critical",
        "label": "Raw Device Mapped disk detected",
        "assessment": "RDM disks are not currently supported by OpenShift Virtualization. The VM cannot be migrated"
    }
}
