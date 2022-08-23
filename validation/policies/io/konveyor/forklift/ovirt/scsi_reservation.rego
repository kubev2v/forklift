package io.konveyor.forklift.ovirt

disks_with_scsi_reservation [i] {
    some i
    input.diskAttachments[i].scsiReservation == true
}

concerns[flag] {
    count(disks_with_scsi_reservation) > 0
    flag := {
        "category": "Warning",
        "label": "Shared disk detected",
        "assessment": "The VM has a disk that is shared. Shared disks are not currently supported by OpenShift Virtualization."
    }
}
