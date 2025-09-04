package io.konveyor.forklift.ovirt

import rego.v1

nics_with_nework_filter_enabled contains i if {
	some i
	input.nics[i].profile.networkFilter != ""
}

concerns contains flag if {
	count(nics_with_nework_filter_enabled) > 0
	flag := {
		"id": "ovirt.nic.network_filter.detected",
		"category": "Warning",
		"label": "NIC with network filter detected",
		"assessment": "The VM is using a vNIC Profile configured with a network filter. These are not currently supported by OpenShift Virtualization.",
	}
}
