package io.konveyor.forklift.ovirt

import rego.v1

default has_ha_enabled := false

has_ha_enabled := value if {
	value := input.haEnabled
}

concerns contains flag if {
	has_ha_enabled
	flag := {
		"id": "ovirt.ha.enabled",
		"category": "Warning",
		"label": "VM configured as HA",
		"assessment": "The VM is configured to be highly available. High availability is not currently supported by OpenShift Virtualization.",
	}
}
