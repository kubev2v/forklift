package io.konveyor.forklift.ovirt

default vm_has_custom_properties = false

vm_has_custom_properties = true {
    count(input.properties) != 0
}

concerns[flag] {
    vm_has_custom_properties
    flag := {
        "category": "Warning",
        "label": "VM custom properties detected",
        "assessment": "The VM is configured with custom properties, which are not currently supported by OpenShift Virtualization."
    }
}
