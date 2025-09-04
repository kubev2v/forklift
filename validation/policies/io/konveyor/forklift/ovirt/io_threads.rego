package io.konveyor.forklift.ovirt

import rego.v1

default has_iothreads_enabled := false

has_iothreads_enabled if {
	input.ioThreads > 1
}

concerns contains flag if {
	has_iothreads_enabled
	flag := {
		"id": "ovirt.iothreads.configured",
		"category": "Information",
		"label": "IO Threads configuration detected",
		"assessment": "The VM is configured to use I/O threads. This configuration will not be automatically applied to the migrated VM, and must be manually re-applied if required.",
	}
}
