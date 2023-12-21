package io.konveyor.forklift.ovirt

default custom_cpu_model = false

custom_cpu_model = true {
    count(input.customCpuModel) != 0
}

concerns[flag] {
    custom_cpu_model
    flag := {
        "category": "Warning",
        "label": "Custom CPU Model detected",
        "assessment": "The VM is configured with a custom CPU model. This configuration will apply to the migrated VM and may not be supported by OpenShift Virtualization."
    }
}