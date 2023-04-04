package io.konveyor.forklift.openstack

image_based_vm {
	input.imageID != ""
}

concerns[flag] {
	image_based_vm
	flag := {
		"category": "Critical",
		"label": "VM is 'Image' based",
		"assessment": "The VM is 'Image' based which is not currently supported. Only the migration of 'Volume' based VMs is supported.",
	}
}
