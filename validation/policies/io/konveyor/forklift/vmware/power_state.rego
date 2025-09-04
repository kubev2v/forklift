package io.konveyor.forklift.vmware

import rego.v1

is_vm_powered_off if {
	input.powerState == "poweredOff"
}

concerns contains flag if {
	is_vm_powered_off
	flag := {
		"id": "vmware.vm_powered_off.detected",
		"category": "Warning",
		"label": "VM is powered off - Static IP preservation requires the VM to be powered on",
		"assessment": "Static IP preservation requires the VM to be powered on.",
	}
}
