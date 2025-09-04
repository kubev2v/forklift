package io.konveyor.forklift.ovirt

import rego.v1

default has_unsupported_os := false

has_unsupported_os if {
	regex.match(`rhel_6|rhel_6x64`, input.osType)
}

concerns contains flag if {
	has_unsupported_os
	flag := {
		"id": "ovirt.os.unsupported",
		"category": "Warning",
		"label": "Unsupported operating system detected",
		"assessment": "The guest operating system is RHEL6 which is not currently supported by OpenShift Virtualization.",
	}
}
