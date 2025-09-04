package io.konveyor.forklift.ova

import rego.v1

default has_hotplug_enabled := false

has_hotplug_enabled if {
	input.cpuHotAddEnabled == true
}

has_hotplug_enabled if {
	input.cpuHotRemoveEnabled == true
}

has_hotplug_enabled if {
	input.memoryHotAddEnabled == true
}

concerns contains flag if {
	has_hotplug_enabled
	flag := {
		"id": "ova.cpu_memory.hotplug.enabled",
		"category": "Warning",
		"label": "CPU/Memory hotplug detected",
		"assessment": "Hot pluggable CPU or memory is not currently supported by Migration Toolkit for Virtualization. You can reconfigure CPU or memory after migration.",
	}
}
