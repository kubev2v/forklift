package io.konveyor.forklift.vmware

has_ha_enabled {
    input.host.cluster.dasEnabled
}

concerns[flag] {
    has_ha_enabled
    flag := {
        "category": "Warning",
        "label": "VM running in HA-enabled cluster",
        "assessment": "Host/Node HA is not currently supported by OpenShift Virtualization. The VM can be migrated but it will not have this feature in the target environment."
    }
}
