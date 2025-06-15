package io.konveyor.forklift.vmware

has_snapshot {
    input.snapshot.kind == "VirtualMachineSnapshot"
}

concerns[flag] {
    has_snapshot
    flag := {
        "id": "vmware.snapshot.detected",
        "category": "Information",
        "label": "VM snapshot detected",
        "assessment": "Online snapshots are not currently supported by OpenShift Virtualization. VM will be migrated with current snapshot."
    }
}
