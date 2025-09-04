package io.konveyor.forklift.ovirt

import rego.v1

default valid_status_string := false

default legal_vm_status := false

valid_status_string if {
	is_string(input.status)
}

legal_vm_status if {
	regex.match(`up|down`, input.status)
}

concerns contains flag if {
	valid_status_string
	not legal_vm_status
	flag := {
		"id": "ovirt.vm.status_invalid",
		"category": "Critical",
		"label": "VM has a status condition that may prevent successful migration",
		"assessment": "The VM's status is not 'up' or 'down'. Attempting to migrate this VM may fail.",
	}
}
