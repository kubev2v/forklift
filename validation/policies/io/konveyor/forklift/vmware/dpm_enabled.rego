package io.konveyor.forklift.vmware

has_dpm_enabled {
    input.host.cluster.dpmEnabled
}

concerns[flag] {
    has_dpm_enabled
    flag := {
        "category": "Information",
        "label": "vSphere DPM detected",
        "assessment": "Distributed Power Management is not currently supported by OpenShift Virtualization. The VM can be migrated but it will not have this feature in the target environment."
    }
}
