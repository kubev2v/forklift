package io.konveyor.forklift.vmware

has_cpu_affinity {
    count(input.cpuAffinity) != 0
}

concerns[flag] {
    has_cpu_affinity
    flag := {
        "id": "vmware.cpu_affinity.detected",
        "category": "Warning",
        "label": "CPU affinity detected",
        "assessment": "The VM will be migrated without CPU affinity, but administrators can set it after migration."
    }
}
