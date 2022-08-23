package io.konveyor.forklift.ovirt

default has_illegal_images = false

has_illegal_images = value {
    value := input.hasIllegalImages
}

concerns[flag] {
    has_illegal_images
    flag := {
        "category": "Critical",
        "label": "Illegal disk images detected",
        "assessment": "The VM has one or more snapshots with disks in ILLEGAL state, which is not currently supported by OpenShift Virtualization. The VM disk transfer is likely to fail."
    }
}
