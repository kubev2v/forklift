package io.konveyor.forklift.openstack

volumes := input.volumes

non_shared_disks[i] {
	some i
	count(volumes[i].attachments) == 1
}

concerns[flag] {
	count(non_shared_disks) != count(volumes)
	flag := {
		"category": "Warning",
		"label": "Shared disk detected",
		"assessment": "The VM has a disk that is shared. Shared disks are not currently supported by OpenShift Virtualization.",
	}
}
