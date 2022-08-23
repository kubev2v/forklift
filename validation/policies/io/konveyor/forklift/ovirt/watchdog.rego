package io.konveyor.forklift.ovirt

default has_watchdog_enabled = false

has_watchdog_enabled = true {
    count(input.watchDogs) != 0
}

concerns[flag] {
    has_watchdog_enabled
    flag := {
        "category": "Warning",
        "label": "Watchdog detected",
        "assessment": "The VM is configured with a watchdog device, which is not currently supported by OpenShift Virtualization. A watchdog device will not be present in the destination VM."
    }
}
