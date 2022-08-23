package io.konveyor.forklift.ovirt

default has_numa_affinity = false

has_numa_affinity = true {
    count(input.numaNodeAffinity) != 0
}

concerns[flag] {
    has_numa_affinity
    flag := {
        "category": "Warning",
        "label": "NUMA tuning detected",
        "assessment": "NUMA tuning is not currently supported by OpenShift Virtualization. The VM can be migrated but it will not have this NUMA mapping in the target environment."
    }
}
