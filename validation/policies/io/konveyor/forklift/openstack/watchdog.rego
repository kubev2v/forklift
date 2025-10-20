package io.konveyor.forklift.openstack

import rego.v1

default has_watchdog_enabled := false

has_watchdog_enabled if "hw:watchdog_action" in object.keys(input.flavor.extraSpecs)

has_watchdog_enabled if input.image.properties.hw_watchdog_action

concerns contains flag if {
	has_watchdog_enabled
	flag := {
		"id": "openstack.watchdog.detected",
		"category": "Warning",
		"label": "Watchdog detected",
		"assessment": "The VM is configured with a watchdog device, which is not currently supported by OpenShift Virtualization. A watchdog device will not be present in the destination VM.",
	}
}
