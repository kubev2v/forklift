package io.konveyor.forklift.ovirt

import rego.v1

default warn_placement_policy := false

warn_placement_policy if {
	regex.match(`\bmigratable\b`, input.placementPolicyAffinity)
}

concerns contains flag if {
	warn_placement_policy
	flag := {
		"id": "ovirt.placement_policy.affinity_set",
		"category": "Warning",
		"label": "Placement policy affinity",
		"assessment": "The VM has a placement policy affinity setting that requires live migration to be enabled in OpenShift Virtualization for compatibility. The target storage classes must also support RWX access mode.",
	}
}
