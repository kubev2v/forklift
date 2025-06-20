package io.konveyor.forklift.ova

default valid_input   = true
default valid_vm      = false
default valid_vm_name = false

valid_input = false {
    is_null(input)
}

valid_vm = true {
    is_string(input.name)
}

valid_vm_name = true {
    regex.match("^(([A-Za-z0-9][-A-Za-z0-9.]*)?[A-Za-z0-9])?$", input.name)
    count(input.name) < 64
}

concerns[flag] {
    valid_input
    valid_vm
    not valid_vm_name
    flag := {
        "id": "ova.name.invalid",
        "category": "Warning",
        "label": "Invalid VM Name",
        "assessment": "The VM name does not comply with the DNS subdomain name format. Edit the name or it will be renamed automatically during the migration to meet RFC 1123. The VM name must be a maximum of 63 characters containing lowercase letters (a-z), numbers (0-9), periods (.), and hyphens (-). The first and last character must be a letter or number. The name cannot contain uppercase letters, spaces or special characters."
    }
}
