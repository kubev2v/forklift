package io.konveyor.forklift.ovirt

import rego.v1

default has_ha_reservation := false

has_ha_reservation := value if {
	value := input.cluster.haReservation
}

concerns contains flag if {
	has_ha_reservation
	flag := {
		"id": "ovirt.ha.reservation.enabled",
		"category": "Warning",
		"label": "Cluster has HA reservation",
		"assessment": "The cluster running the source VM has a resource reservation to allow highly available VMs to be started. This feature is not currently supported by OpenShift Virtualization.",
	}
}
