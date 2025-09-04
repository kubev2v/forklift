package io.konveyor.forklift.ovirt

import rego.v1

default has_watchdog_enabled := false

has_watchdog_enabled if {
	count(input.watchDogs) != 0
}

concerns contains flag if {
	has_watchdog_enabled
	flag := {
		"id": "ovirt.watchdog.enabled",
		"category": "Warning",
		"label": "Watchdog detected",
		"assessment": "The VM is configured with a watchdog device, which is not currently supported by OpenShift Virtualization. A watchdog device will not be present in the destination VM.",
	}
}
