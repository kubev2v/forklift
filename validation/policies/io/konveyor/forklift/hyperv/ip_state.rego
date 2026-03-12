package io.konveyor.forklift.hyperv

import rego.v1

default has_guest_networks := false

has_guest_networks if {
	count(input.guestNetworks) > 0
}

is_vm_missing_ip if {
	not has_guest_networks
}

concerns contains flag if {
	is_vm_missing_ip
	flag := {
		"id": "hyperv.vm_missing_ip.detected",
		"category": "Warning",
		"label": "VM is missing IP addresses",
		"assessment": "To collect IP addresses for static IP preservation, power on the VM with Integration Services active.",
	}
}
