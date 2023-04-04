package io.konveyor.forklift.openstack

default host_devices = false

host_devices {
	input.flavor.extraSpecs["pci_passthrough:alias"]
}

concerns[flag] {
	host_devices
	flag := {
		"category": "Warning",
		"label": "VM has mapped host devices",
		"assessment": "The VM is configured with hardware devices mapped from the host. This functionality is not currently supported by OpenShift Virtualization.",
	}
}
