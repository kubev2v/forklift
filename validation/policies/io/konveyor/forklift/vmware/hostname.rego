package io.konveyor.forklift.vmware

import rego.v1

is_empty_hostname if {
	input.hostName == ""
}

is_localhost_hostname if {
	input.hostName == "localhost.localdomain"
}

concerns contains flag if {
	is_empty_hostname
	flag := {
		"id": "vmware.hostname.empty",
		"category": "Warning",
		"label": "Empty Host Name",
		"assessment": "The 'hostname' field is missing or empty. The hostname might be renamed during migration.",
	}
}

concerns contains flag if {
	is_localhost_hostname
	flag := {
		"id": "vmware.hostname.default",
		"category": "Warning",
		"label": "Default Host Name",
		"assessment": "The 'hostname' is set to 'localhost.localdomain', which is a default value. The hostname might be renamed during migration.",
	}
}
