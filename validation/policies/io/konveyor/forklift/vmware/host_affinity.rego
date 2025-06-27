package io.konveyor.forklift.vmware

has_host_affinity {
    some i
    input.host.cluster.hostAffinityVms[i].id == input.id
}

concerns[flag] {
    has_host_affinity
    flag := {
        "id": "vmware.host_affinity.detected",
        "category": "Warning",
        "label": "VM-Host affinity detected",
        "assessment": "The VM will be migrated without node affinity, but administrators can set it after migration."
    }
}
