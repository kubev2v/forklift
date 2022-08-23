package io.konveyor.forklift.ovirt

nics_with_port_mirroring_enabled [i] {
    some i
    input.nics[i].profile.portMirroring == true
}

concerns[flag] {
    count(nics_with_port_mirroring_enabled) > 0
    flag := {
        "category": "Warning",
        "label": "NIC with port mirroring detected",
        "assessment": "The VM is using a vNIC Profile configured with port mirroring. This is not currently supported by OpenShift Virtualization."
    }
}
