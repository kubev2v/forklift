package io.konveyor.forklift.ovirt

default has_cpushares_enabled = false

has_cpushares_enabled = true {
    input.cpuShares > 0
}

concerns[flag] {
    has_cpushares_enabled
    flag := {
        "category": "Warning",
        "label": "VM has CPU Shares Defined",
        "assessment": "The VM has CPU shares defined. This functionality is not currently supported by OpenShift Virtualization. The VM can be migrated but the CPU shares configuration will be missing in the target environment."
    }
}
