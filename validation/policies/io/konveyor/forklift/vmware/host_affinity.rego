package io.konveyor.forklift.vmware

has_host_affinity {
    some i
    input.host.cluster.hostAffinityVms[i].id == input.id
}

concerns[flag] {
    has_host_affinity
    flag := {
        "category": "Warning",
        "label": "VM-Host affinity detected",
        "assessment": "VM-Host affinity is not currently supported by OpenShift Virtualization. The VM can be migrated but it will not have this feature in the target environment."
    }
}
