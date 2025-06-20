package io.konveyor.forklift.vmware

import future.keywords.in

is_empty_hostname {
    input.hostName == ""
}


is_localhost_hostname {
    input.hostName == "localhost.localdomain"
}

concerns[flag] {
    is_empty_hostname
    flag := {
        "id": "vmware.hostname.empty",
        "category": "Warning",
        "label": "Empty Host Name",
        "assessment": "The 'hostname' field is missing or empty. The hostname might be renamed during migration."
    }
}


concerns[flag] {
    is_localhost_hostname
    flag := {
        "id": "vmware.hostname.default",
        "category": "Warning",
        "label": "Default Host Name",
        "assessment": "The 'hostname' is set to 'localhost.localdomain', which is a default value. The hostname might be renamed during migration."
    }
}