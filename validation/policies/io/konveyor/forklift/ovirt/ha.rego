package io.konveyor.forklift.ovirt

default has_ha_enabled = false

has_ha_enabled = value {
   value :=  input.haEnabled
}

concerns[flag] {
    has_ha_enabled
    flag := {
        "category": "Warning",
        "label": "VM configured as HA",
        "assessment": "The VM is configured to be highly available. High availability is not currently supported by OpenShift Virtualization."
    }
}
