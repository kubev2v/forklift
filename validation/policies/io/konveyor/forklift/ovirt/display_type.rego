package io.konveyor.forklift.ovirt

import rego.v1

default has_spice_display_enabled := false

has_spice_display_enabled if {
	input.display == "spice"
}

concerns contains flag if {
	has_spice_display_enabled
	flag := {
		"id": "ovirt.display_type.spice.enabled",
		"category": "Information",
		"label": "VM Display Type",
		"assessment": "The VM is using the SPICE protocol for video display. This is not supported by OpenShift Virtualization.",
	}
}
