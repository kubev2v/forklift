package io.konveyor.forklift.vmware
import future.keywords.in

independent_disk {
    some i
    input.disks[i].mode in ["independent_persistent", "independent_nonpersistent"]
}

concerns[flag] {
    independent_disk
    flag := {
        "category": "Critical",
        "label": "Independent disk detected",
        "assessment": "Independent disks cannot be transferred using recent versions of VDDK. The VM cannot be migrated unless disks are changed to 'Dependent' mode in VMware."
    }
}
