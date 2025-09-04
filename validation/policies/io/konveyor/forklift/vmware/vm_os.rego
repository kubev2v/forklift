package io.konveyor.forklift.vmware

import rego.v1

default has_unsupported_os := false

unsupported_os_name_substrings := [
	"red hat enterprise linux 6",
	"vmware photon os",
]

has_unsupported_os if {
	lower_id := lower(input.guestId)
	regex.match(`.*(rhel6guest|rhel6_64guest|photonguest|photon64guest).*`, lower_id)
}

has_unsupported_os if {
	lower_name := lower(input.guestNameFromVmwareTools)
	some i
	contains(lower_name, unsupported_os_name_substrings[i])
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
