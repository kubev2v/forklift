package io.konveyor.forklift.openstack

default has_watchdog_enabled = false

has_watchdog_enabled {
	input.flavor.extraSpecs["hw:watchdog_action"]
}

has_watchdog_enabled {
	input.image.properties.hw_watchdog_action
}

concerns[flag] {
	has_watchdog_enabled
	flag := {
		"category": "Warning",
		"label": "Watchdog detected",
		"assessment": "The VM is configured with a watchdog device, which is not currently supported by OpenShift Virtualization. A watchdog device will not be present in the destination VM.",
	}
}
