package io.konveyor.forklift.ovirt

default has_unsupported_os = false

has_unsupported_os = true {
    regex.match(`rhel_6|rhel_6x64`, input.osType)
}

concerns[flag] {
    has_unsupported_os
    flag := {
        "id": "ovirt.os.unsupported",
        "category": "Warning",
        "label": "Unsupported operating system detected",
        "assessment": "The guest operating system is RHEL6 which is not currently supported by OpenShift Virtualization."
    }
}
