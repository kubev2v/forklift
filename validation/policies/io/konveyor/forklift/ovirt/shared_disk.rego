package io.konveyor.forklift.ovirt

shared_disks [i] {
    some i
    input.diskAttachments[i].disk.shared == true
}

concerns[flag] {
    count(shared_disks) > 0
    flag := {
        "category": "Warning",
        "label": "Shared disk detected",
        "assessment": "The VM has a disk that is shared. Shared disks are not currently supported by OpenShift Virtualization."
    }
}
