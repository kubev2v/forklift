package io.konveyor.forklift.openstack

import future.keywords.if

default invalid_disk_interface = false

invalid_disk_interface if {
	not regex.match(`sata|scsi|virtio`, input.image.properties.hw_disk_bus)
}

concerns[flag] {
	invalid_disk_interface
	flag := {
	    "id": "openstack.disk.unsupported_interface",
		"category": "Warning",
		"label": "Unsupported disk interface type detected",
		"assessment": "The disk interface type is not supported by OpenShift Virtualization (only sata, scsi and virtio interface types are currently supported). The migrated VM will be given a virtio disk interface type.",
	}
}
