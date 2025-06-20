package io.konveyor.forklift.openstack

import future.keywords.if
import future.keywords.in

default has_numa_enabled = false

has_numa_enabled if "hw:pci_numa_affinity_policy" in object.keys(input.flavor.extraSpecs)

has_numa_enabled if "hw:numa_nodes" in object.keys(input.flavor.extraSpecs)

concerns[flag] {
	has_numa_enabled
	flag := {
		"id": "openstack.numa_tuning.detected",
		"category": "Warning",
		"label": "NUMA tuning detected",
		"assessment": "NUMA tuning is not currently supported by OpenShift Virtualization. The VM can be migrated but it will not have this NUMA mapping in the target environment.",
	}
}
