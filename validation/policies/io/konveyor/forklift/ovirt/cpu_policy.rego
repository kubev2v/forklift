package io.konveyor.forklift.ovirt

default not_supported_cpu_policy = false

# no need to check for 'manual' pinning policy because 'cpuAffinity' is validated by the 'cpu_tune' policy
not_supported_cpu_policy = true {
    regex.match(`resize_and_pin_numa|isolate_threads`, input.cpuPinningPolicy)
}

concerns[flag] {
    not_supported_cpu_policy
    flag := {
        "category": "Warning",
        "label": "Unsupported CPU pinning policy detected",
        "assessment": "Resize and Pin NUMA and Isolated Threads are not supported by OpenShift Virtualization. Some functionality may be missing after the VM is migrated."
    }
}
