package io.konveyor.forklift.ovirt

import rego.v1

valid_nic_interfaces contains i if {
	some i
	regex.match(`e1000|rtl8139|virtio`, input.nics[i].interface)
}

number_of_nics contains i if {
	some i
	input.nics[i].id
}

concerns contains flag if {
	count(valid_nic_interfaces) != count(number_of_nics)
	flag := {
		"id": "ovirt.nic.interface_type.unsupported",
		"category": "Warning",
		"label": "Unsupported NIC interface type detected",
		"assessment": "The NIC interface type is not supported by OpenShift Virtualization (only e1000, rtl8139 and virtio interface types are currently supported). The migrated VM will be given a virtio NIC interface type.",
	}
}
