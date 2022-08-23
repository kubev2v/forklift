package io.konveyor.forklift.vmware

has_drs_enabled {
    input.host.cluster.drsEnabled
}

concerns[flag] {
    has_drs_enabled
    flag := {
        "category": "Information",
        "label": "VM running in a DRS-enabled cluster",
        "assessment": "Distributed resource scheduling is not currently supported by OpenShift Virtualization. The VM can be migrated but it will not have this feature in the target environment."
    }
}
