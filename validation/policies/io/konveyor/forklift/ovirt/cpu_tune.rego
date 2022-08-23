package io.konveyor.forklift.ovirt

default has_cpu_affinity = false

has_cpu_affinity = true {
    count(input.cpuAffinity) != 0
}

concerns[flag] {
    has_cpu_affinity
    flag := {
        "category": "Warning",
        "label": "CPU tuning detected",
        "assessment": "CPU tuning other than 1 vCPU - 1 pCPU is not currently supported by OpenShift Virtualization. The VM can be migrated but it will not have this feature in the target environment."
    }
}
