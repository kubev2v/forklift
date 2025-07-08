package io.konveyor.forklift.ovirt

unplugged_nics [i] {
    some i
    input.nics[i].plugged == false
}

concerns[flag] {
    count(unplugged_nics) > 0
    flag := {
        "id": "ovirt.nic.unplugged.detected",
        "category": "Warning",
        "label": "Unplugged NIC detected",
        "assessment": "The VM has a NIC that is unplugged from a network. This is not currently supported by OpenShift Virtualization."
    }
}
