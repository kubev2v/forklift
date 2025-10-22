package io.konveyor.forklift.vmware

has_rdm_disk {
    some i
    input.disks[i].rdm
}

concerns[flag] {
    has_rdm_disk
    flag := {
        "id": "vmware.disk.rdm.detected",
        "category": "Critical",
        "label": "Raw Device Mapped disk detected",
        "assessment": "RDM disks are not currently supported by Migration Toolkit for Virtualization. The VM cannot be migrated unless the RDM disks are removed. You can reattach them to the VM after migration."
    }
}
