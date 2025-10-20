package io.konveyor.forklift.ovirt

import rego.v1

valid_disk_interfaces contains i if {
	some i
	regex.match(`sata|virtio_scsi|virtio`, input.diskAttachments[i].interface)
}

number_of_disks contains i if {
	some i
	input.diskAttachments[i].id
}

concerns contains flag if {
	count(valid_disk_interfaces) != count(number_of_disks)
	flag := {
		"id": "ovirt.disk.interface_type.unsupported",
		"category": "Warning",
		"label": "Unsupported disk interface type detected",
		"assessment": "The disk interface type is not supported by OpenShift Virtualization (only sata, virtio_scsi and virtio interface types are currently supported). The migrated VM will be given a virtio disk interface type.",
	}
}
