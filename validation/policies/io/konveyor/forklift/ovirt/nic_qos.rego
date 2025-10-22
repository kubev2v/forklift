package io.konveyor.forklift.ovirt

nics_with_qos_enabled [i] {
    some i
    input.nics[i].profile.qos != ""
}

concerns[flag] {
    count(nics_with_qos_enabled) > 0
    flag := {
        "id": "ovirt.nic.qos.detected",
        "category": "Warning",
        "label": "NIC with QoS settings detected",
        "assessment": "The VM has a vNIC Profile that includes Quality of Service settings. This is not currently supported by OpenShift Virtualization."
    }
}
