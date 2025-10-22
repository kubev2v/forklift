package io.konveyor.forklift.vmware

has_numa_node_affinity {
    count(input.numaNodeAffinity) != 0
}

concerns[flag] {
    has_numa_node_affinity
    flag := {
        "id": "vmware.numa_affinity.detected",
        "category": "Warning",
        "label": "NUMA node affinity detected",
        "assessment": "NUMA node affinity is not currently supported by Migration Toolkit for Virtualization. The VM can be migrated but it will not have this feature in the target environment."
    }
}
