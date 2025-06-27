package io.konveyor.forklift.vmware

has_fault_tolerance_enabled {
    input.faultToleranceEnabled
}

concerns[flag] {
    has_fault_tolerance_enabled
    flag := {
        "id": "vmware.fault_tolerance.enabled",
        "category": "Information",
        "label": "Fault tolerance",
        "assessment": "Fault tolerance is not currently supported by OpenShift Virtualization. The VM can be migrated but it will not have this feature in the target environment."
    }
}
