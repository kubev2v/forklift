package io.konveyor.forklift.vmware
import future.keywords.in

independent_disk {
    some i
    input.disks[i].mode in ["independent_persistent", "independent_nonpersistent"]
}

concerns[flag] {
    independent_disk
    flag := {
        "category": "Warning",
        "label": "Independent disk detected",
        "assessment": "Independent disks cannot be transferred using recent versions of VDDK. It is recommended to change them in vSphere to 'Dependent' mode, or alternatively, to export the VM to an OVA."
    }
}
