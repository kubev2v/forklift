# NIC network resolution validation for Hyper-V VMs.
# Detects NICs that reference a virtual switch by name but whose network UUID
# could not be resolved during inventory collection. This typically happens in
# cluster environments where a switch exists on a remote node but not on the
# provider's connected host.

package io.konveyor.forklift.hyperv

import rego.v1

nics_with_unresolved_network contains idx if {
	some idx
	nic := input.nics[idx]
	has_network_name(nic)
	not has_network_uuid(nic)
}

has_network_name(nic) if {
	is_string(nic.networkName)
	nic.networkName != ""
}

has_network_uuid(nic) if {
	is_string(nic.network.id)
	nic.network.id != ""
}

concerns contains flag if {
	nics_with_unresolved_network[idx]
	nic := input.nics[idx]
	flag := {
		"id": "hyperv.nic.unresolved_network",
		"category": "Warning",
		"label": sprintf("NIC '%v' references undiscovered switch '%v'", [nic.name, nic.networkName]),
		"assessment": sprintf("NIC '%v' is connected to virtual switch '%v' which was not found in the provider's network inventory. In a cluster, this switch may only exist on a remote node. The NIC will be skipped during migration unless the switch is created on the provider's connected host or all cluster nodes are reachable. Verify that the virtual switch exists and is accessible.", [nic.name, nic.networkName]),
	}
}
