package io.konveyor.forklift.ovirt

import rego.v1

default has_host_devices := false

has_host_devices if {
	count(input.hostDevices) != 0
}

concerns contains flag if {
	has_host_devices
	flag := {
		"id": "ovirt.host_devices.mapped",
		"category": "Warning",
		"label": "VM has mapped host devices",
		"assessment": "The VM is configured with hardware devices mapped from the host. This functionality is not currently supported by OpenShift Virtualization. The VM can be migrated but it will not have any host device attached to it in the target environment.",
	}
}
