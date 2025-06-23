package io.konveyor.forklift.vmware

default has_unsupported_os = false

has_unsupported_os = true {
    lower_id := lower(input.guestId)
    regex.match(`.*(rhel6guest|rhel6_64guest|photonguest|photon64guest).*`, lower_id)
}

concerns[flag] {
    has_unsupported_os
    flag := {
        "id": "vmware.os.unsupported",
        "category":   "Warning",
        "label":      "Unsupported operating system detected",
        "assessment": "The guest operating system is not currently supported by the Migration Toolkit for Virtualization"
    }
}
