# Supported OS list based on Red Hat virt-v2v documentation:
# https://access.redhat.com/articles/1351473

package io.konveyor.forklift.hyperv

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

is_supported if {
	is_string(input.guestOS)
	lower_name := lower(input.guestOS)
	some i
	lower_substring := lower(supported_os_name_substrings[i])
	contains(lower_name, lower_substring)
}

has_unsupported_os if {
	has_guest_os
	not is_supported
}

has_guest_os if {
	is_string(input.guestOS)
	input.guestOS != ""
}

concerns contains flag if {
	has_unsupported_os
	flag := {
		"id": "hyperv.os.unsupported",
		"category": "Warning",
		"label": "Unsupported operating system detected",
		"assessment": "The guest operating system is not currently supported by the Migration Toolkit for Virtualization.",
	}
}
