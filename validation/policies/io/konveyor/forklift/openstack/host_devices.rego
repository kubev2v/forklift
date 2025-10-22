package io.konveyor.forklift.openstack

import future.keywords.if
import future.keywords.in

default host_devices = false

host_devices if "pci_passthrough:alias" in object.keys(input.flavor.extraSpecs)

concerns[flag] {
	host_devices
	flag := {
		"id": "openstack.host_devices.mapped",
		"category": "Warning",
		"label": "VM has mapped host devices",
		"assessment": "The VM is configured with hardware devices mapped from the host. This functionality is not currently supported by OpenShift Virtualization. The VM can be migrated but it will not have any host device attached to it in the target environment.",
	}
}
