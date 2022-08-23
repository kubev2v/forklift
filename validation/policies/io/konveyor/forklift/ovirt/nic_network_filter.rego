package io.konveyor.forklift.ovirt

nics_with_nework_filter_enabled [i] {
    some i
    input.nics[i].profile.networkFilter != ""
}

concerns[flag] {
    count(nics_with_nework_filter_enabled) > 0
    flag := {
        "category": "Warning",
        "label": "NIC with network filter detected",
        "assessment": "The VM is using a vNIC Profile configured with a network filter. These are not currently supported by OpenShift Virtualization."
    }
}
