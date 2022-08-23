package io.konveyor.forklift.vmware

has_numa_node_affinity {
    count(input.numaNodeAffinity) != 0
}

concerns[flag] {
    has_numa_node_affinity
    flag := {
        "category": "Warning",
        "label": "NUMA node affinity detected",
        "assessment": "NUMA node affinity is not currently supported by OpenShift Virtualization. The VM can be migrated but it will not have this feature in the target environment."
    }
}
