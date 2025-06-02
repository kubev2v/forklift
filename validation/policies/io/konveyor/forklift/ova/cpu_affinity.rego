package io.konveyor.forklift.ova

has_cpu_affinity {
    count(input.cpuAffinity) != 0
}

concerns[flag] {
    has_cpu_affinity
    flag := {
        "category": "Warning",
        "label": "CPU affinity detected",
        "assessment": "The VM will be migrated without CPU affinity, but administrators can set it after migration."
    }
}
