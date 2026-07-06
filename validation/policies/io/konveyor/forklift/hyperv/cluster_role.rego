package io.konveyor.forklift.hyperv

import rego.v1

default is_cluster_mode := false
default is_cluster_role := false

is_cluster_mode if {
	input.managementType == "cluster"
}


is_cluster_role if {
	input.isClusterRole
}

concerns contains flag if {
	is_cluster_mode
	not is_cluster_role
	flag := {
		"id": "hyperv.cluster_role.not_registered",
		"category": "Warning",
		"label": "VM is not a Failover Cluster role",
		"assessment": "This VM exists on a cluster node but is not registered as a Failover Cluster role. It will not fail over automatically if its host goes down. The VM can still be migrated.",
	}
}
