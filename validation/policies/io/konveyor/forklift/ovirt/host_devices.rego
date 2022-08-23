package io.konveyor.forklift.ovirt

default has_host_devices = false

has_host_devices = true {
    count(input.hostDevices) != 0
}

concerns[flag] {
    has_host_devices
    flag := {
        "category": "Warning",
        "label": "VM has mapped host devices",
        "assessment": "The VM is configured with hardware devices mapped from the host. This functionality is not currently supported by OpenShift Virtualization."
    }
}
