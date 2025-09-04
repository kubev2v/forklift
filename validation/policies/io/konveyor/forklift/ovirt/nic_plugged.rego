package io.konveyor.forklift.ovirt

import rego.v1

unplugged_nics contains i if {
	some i
	input.nics[i].plugged == false
}

concerns contains flag if {
	count(unplugged_nics) > 0
	flag := {
		"id": "ovirt.nic.unplugged.detected",
		"category": "Warning",
		"label": "Unplugged NIC detected",
		"assessment": "The VM has a NIC that is unplugged from a network. This is not currently supported by OpenShift Virtualization.",
	}
}
