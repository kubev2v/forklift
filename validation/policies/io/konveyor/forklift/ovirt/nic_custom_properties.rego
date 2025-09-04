package io.konveyor.forklift.ovirt

import rego.v1

default vnic_has_custom_properties := false

vnic_has_custom_properties if {
	count(input.nics[i].profile.properties) != 0
}

concerns contains flag if {
	vnic_has_custom_properties
	flag := {
		"id": "ovirt.nic.custom_properties.detected",
		"category": "Warning",
		"label": "vNIC custom properties detected",
		"assessment": "The VM's vNIC Profile is configured with custom properties, which are not currently supported by OpenShift Virtualization.",
	}
}
