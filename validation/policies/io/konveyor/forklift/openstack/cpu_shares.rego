package io.konveyor.forklift.openstack

import future.keywords.if
import future.keywords.in

default has_cpushares_enabled = false

has_cpushares_enabled if "quota:cpu_shares" in object.keys(input.flavor.extraSpecs)

concerns[flag] {
	has_cpushares_enabled
	flag := {
		"category": "Information",
		"label": "VM has CPU Shares Defined",
		"assessment": "The VM has CPU shares defined. This functionality is not currently supported by OpenShift Virtualization.",
	}
}
