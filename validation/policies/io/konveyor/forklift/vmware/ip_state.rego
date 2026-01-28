package io.konveyor.forklift.vmware

import rego.v1

is_vm_missing_ip if {
	count(input.guestNetworks) == 0
}

concerns contains flag if {
	is_vm_missing_ip
	flag := {
		"id": "vmware.vm_missing_ip.detected",
		"category": "Warning",
		"label": "VM is missing IP addresses",
		"assessment": "Static IP preservation requires the VM to be powered on and running VMware tools.",
	}
}
