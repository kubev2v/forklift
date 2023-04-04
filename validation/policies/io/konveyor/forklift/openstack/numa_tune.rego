package io.konveyor.forklift.openstack

default has_numa_enabled = false

has_numa_enabled {
	input.flavor.extraSpecs["hw:pci_numa_affinity_policy"]
}

has_numa_enabled {
	input.flavor.extraSpecs["hw:numa_nodes"]
}

concerns[flag] {
	has_numa_enabled
	flag := {
		"category": "Warning",
		"label": "NUMA tuning detected",
		"assessment": "NUMA tuning is not currently supported by OpenShift Virtualization. The VM can be migrated but it will not have this NUMA mapping in the target environment.",
	}
}
