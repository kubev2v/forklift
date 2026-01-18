# Supported OS list based on Red Hat virt-v2v documentation:
# https://access.redhat.com/articles/1351473

package io.konveyor.forklift.vmware

import rego.v1

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

supported_os_regex := `.*(rhel7guest|rhel7_64guest|rhel8guest|rhel8_64guest|rhel9guest|rhel9_64guest|rhel10guest|rhel10_64guest|windows10.*guest|windows11.*guest|windows2016.*guest|windows2019.*guest|windows2022.*guest|windows2025.*guest).*`

is_supported if {
    # 1. Check guestId against regex
    is_string(input.guestId)
    lower_id := lower(input.guestId)
    regex.match(supported_os_regex, lower_id)
} else if {
    # 2. Check guestNameFromVmwareTools
    is_string(input.guestNameFromVmwareTools)
    lower_name := lower(input.guestNameFromVmwareTools)
    some i
    lower_substring := lower(supported_os_name_substrings[i])
    contains(lower_name, lower_substring)
} else if {
    # 3. Check guestName if guestNameFromVmwareTools is missing
    is_string(input.guestName)
    not is_string(input.guestNameFromVmwareTools)
    lower_name := lower(input.guestName)
    some i
    lower_substring := lower(supported_os_name_substrings[i])
    contains(lower_name, lower_substring)
} else if {
    # 4. Check guestName if guestNameFromVmwareTools exists but is empty
    is_string(input.guestName)
    is_string(input.guestNameFromVmwareTools)
    input.guestNameFromVmwareTools == ""
    lower_name := lower(input.guestName)
    some i
    lower_substring := lower(supported_os_name_substrings[i])
    contains(lower_name, lower_substring)
}


has_unsupported_os if {
    has_guest_id_or_name
    not is_supported
}

has_guest_id_or_name if {
    is_string(input.guestId)
} else if {
    is_string(input.guestNameFromVmwareTools)
    input.guestNameFromVmwareTools != ""
} else if {
    is_string(input.guestName)
    not is_string(input.guestNameFromVmwareTools)
} else if {
    is_string(input.guestName)
    is_string(input.guestNameFromVmwareTools)
    input.guestNameFromVmwareTools == ""
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
