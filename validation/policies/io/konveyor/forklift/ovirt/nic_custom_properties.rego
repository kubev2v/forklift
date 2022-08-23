package io.konveyor.forklift.ovirt

default vnic_has_custom_properties = false

vnic_has_custom_properties = true {
    count(input.nics[i].profile.properties) != 0
}

concerns[flag] {
    vnic_has_custom_properties
    flag := {
        "category": "Warning",
        "label": "vNIC custom properties detected",
        "assessment": "The VM's vNIC Profile is configured with custom properties, which are not currently supported by OpenShift Virtualization."
    }
}
