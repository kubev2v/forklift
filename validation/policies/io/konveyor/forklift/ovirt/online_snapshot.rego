package io.konveyor.forklift.ovirt

online_snapshots [i] {
    some i
    input.snapshots[i].persistMemory
}

concerns[flag] {
    count(online_snapshots) > 0
    flag := {
        "id": "ovirt.snapshot.online_memory.detected",
        "category": "Warning",
        "label": "Online (memory) snapshot detected",
        "assessment": "The VM has a snapshot that contains a memory copy. Online snapshots such as this are not curently supported by OpenShift Virtualization."
    }
}
