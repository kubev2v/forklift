package io.konveyor.forklift.ovirt

invalid_disk_status [i] {
    some i
    regex.match(`illegal|locked`, input.diskAttachments[i].disk.status)
}

concerns[flag] {
    count(invalid_disk_status) > 0
    flag := {
        "category": "Critical",
        "label": "VM has an illegal or locked disk status condition",
        "assessment": "One or more of the VM's disks has an illegal or locked status condition. The VM disk transfer is likely to fail."
    }
}
