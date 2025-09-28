package io.konveyor.forklift.openstack

import rego.v1

addresses := input.addresses

floating_ips contains i if {
	some i
	addresses[i][_]["OS-EXT-IPS:type"] == "floating"
}

concerns contains flag if {
	count(floating_ips) != 0
	flag := {
		"id": "openstack.network.floating_ips.detected",
		"category": "Warning",
		"label": "Floating IPs detected",
		"assessment": "The VM has floating IPs assigned. This functionality is not currently supported by OpenShift Virtualization. The VM can be migrated but the Floating IP configuration will be missing in the target environment.",
	}
}
