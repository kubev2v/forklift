package io.konveyor.forklift.ovirt

default valid_status_string = false
default legal_vm_status = false

valid_status_string = true {
    is_string(input.status)
}

legal_vm_status = true {
    regex.match(`up|down`, input.status)
}

concerns[flag] {
    valid_status_string
    not legal_vm_status
    flag := {
        "category": "Critical",
        "label": "VM has a status condition that may prevent successful migration",
        "assessment": "The VM's status is not 'up' or 'down'. Attempting to migrate this VM may fail."
    }
}
