package io.konveyor.forklift.openstack

import rego.v1

default has_cpushares_enabled := false

has_cpushares_enabled if "quota:cpu_shares" in object.keys(input.flavor.extraSpecs)

concerns contains flag if {
	has_cpushares_enabled
	flag := {
		"id": "openstack.cpu.shares.defined",
		"category": "Warning",
		"label": "VM has CPU Shares Defined",
		"assessment": "The VM has CPU shares defined. This functionality is not currently supported by OpenShift Virtualization. The VM can be migrated but the CPU shares configuration will be missing in the target environment.",
	}
}
