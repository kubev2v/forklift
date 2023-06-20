package io.konveyor.forklift.openstack

addresses := input.addresses

floating_ips[i] {
  some i
	addresses[i][_]["OS-EXT-IPS:type"] == "floating"
}

concerns[flag] {
	count(floating_ips) != 0
	flag := {
		"category": "Warning",
		"label": "Floating IPs detected",
		"assessment": "The VM has floating IPs assigned. This functionality is not currently supported by OpenShift Virtualization. The VM can be migrated but the Floating IP configuration will be missing in the target environment.",
	}
}
