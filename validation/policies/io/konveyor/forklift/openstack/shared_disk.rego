package io.konveyor.forklift.openstack

volumes := input.volumes

shared_disks[i] {
	some i
	count(volumes[i].attachments) > 1
}

concerns[flag] {
	count(shared_disks) > 0
	flag := {
	    "id": "openstack.disk.shared.detected",
		"category": "Warning",
		"label": "Shared disk detected",
		"assessment": "The VM has a disk that is shared. Shared disks are not currently supported by OpenShift Virtualization.",
	}
}
