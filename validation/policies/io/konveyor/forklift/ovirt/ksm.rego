package io.konveyor.forklift.ovirt

import rego.v1

default has_ksm_enabled := false

has_ksm_enabled := value if {
	value := input.cluster.ksmEnabled
}

concerns contains flag if {
	has_ksm_enabled
	flag := {
		"id": "ovirt.cluster.ksm_enabled",
		"category": "Warning",
		"label": "Cluster has KSM enabled",
		"assessment": "The host running the source VM has kernel samepage merging enabled for more efficient memory utilization. This feature is not currently supported by OpenShift Virtualization.",
	}
}
