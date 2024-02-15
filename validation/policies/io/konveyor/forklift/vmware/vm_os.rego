package io.konveyor.forklift.vmware

default has_unsupported_os = false

has_unsupported_os = true {
    regex.match(`rhel6Guest|rhel6_64Guest`, input.guestId)
}

concerns[flag] {
    has_unsupported_os
    flag := {
        "category": "Warning",
        "label": "Unsupported operating system detected",
        "assessment": "The guest operating system is RHEL6 which is not currently supported by OpenShift Virtualization."
    }
}
