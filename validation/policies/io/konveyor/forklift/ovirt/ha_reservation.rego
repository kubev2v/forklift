package io.konveyor.forklift.ovirt

default has_ha_reservation = false

has_ha_reservation = value {
    value := input.cluster.haReservation
}

concerns[flag] {
    has_ha_reservation
    flag := {
        "category": "Warning",
        "label": "Cluster has HA reservation",
        "assessment": "The cluster running the source VM has a resource reservation to allow highly available VMs to be started. This feature is not currently supported by OpenShift Virtualization."
    }
}
