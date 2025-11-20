package io.konveyor.forklift.vmware

import rego.v1

# Supported OS list based on Red Hat virt-v2v documentation:
# https://access.redhat.com/articles/1351473

default has_unsupported_os := false

supported_os_name_substrings := [
    "red hat enterprise linux 7",
    "red hat enterprise linux 8",
    "red hat enterprise linux 9",
	"red hat enterprise linux 10",
	"Windows 10",
	"Windows 11",
	"Windows Server 2016",
	"Windows Server 2019",
	"Windows Server 2022",
	"Windows Server 2025",
]

supported_os_regex := `.*(rhel7guest|rhel7_64guest|rhel8guest|rhel8_64guest|rhel9guest|rhel9_64guest|rhel10guest|rhel10_64guest|` +
	`windows10.*guest|windows11.*guest|windows2016.*guest|windows2019.*guest|windows2022.*guest|windows2025.*guest).*`

is_supported if {
    lower_id := lower(input.guestId)
    regex.match(supported_os_regex, lower_id)
}

is_supported if {
    lower_name := lower(input.guestNameFromVmwareTools)
    some i
    contains(lower_name, supported_os_name_substrings[i])
}

has_unsupported_os if {
    not is_supported
}

concerns contains flag if {
    has_unsupported_os
    flag := {
        "id": "vmware.os.unsupported",
        "category": "Warning",
        "label": "Unsupported operating system detected",
        "assessment": "The guest operating system is not currently supported by the Migration Toolkit for Virtualization",
    }
}
