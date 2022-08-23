package io.konveyor.forklift.vmware

has_snapshot {
    input.snapshot.kind == "VirtualMachineSnapshot"
}

concerns[flag] {
    has_snapshot
    flag := {
        "category": "Information",
        "label": "VM snapshot detected",
        "assessment": "Online snapshots are not currently supported by OpenShift Virtualization."
    }
}
