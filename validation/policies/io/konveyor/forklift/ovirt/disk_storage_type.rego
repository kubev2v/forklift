package io.konveyor.forklift.ovirt

import rego.v1

valid_disk_storage_type contains i if {
	some i
	input.diskAttachments[i].disk.storageType == "image"
}

valid_disk_storage_type_lun contains i if {
	some i
	input.diskAttachments[i].disk.storageType == "lun"
}

concerns contains flag if {
	count(valid_disk_storage_type) + count(valid_disk_storage_type_lun) != count(number_of_disks)
	flag := {
		"id": "ovirt.disk.storage_type.unsupported",
		"category": "Critical",
		"label": "Unsupported disk storage type detected",
		"assessment": "The VM has a disk with a storage type other than 'image' or 'lun', which is not currently supported by OpenShift Virtualization. The VM disk transfer is likely to fail.",
	}
}
