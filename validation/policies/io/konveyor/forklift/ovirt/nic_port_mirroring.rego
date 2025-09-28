package io.konveyor.forklift.ovirt

import rego.v1

nics_with_port_mirroring_enabled contains i if {
	some i
	input.nics[i].profile.portMirroring == true
}

concerns contains flag if {
	count(nics_with_port_mirroring_enabled) > 0
	flag := {
		"id": "ovirt.nic.port_mirroring.detected",
		"category": "Warning",
		"label": "NIC with port mirroring detected",
		"assessment": "The VM is using a vNIC Profile configured with port mirroring. This is not currently supported by OpenShift Virtualization.",
	}
}
