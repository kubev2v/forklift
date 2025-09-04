package io.konveyor.forklift.ovirt

import rego.v1

default has_illegal_images := false

has_illegal_images := value if {
	value := input.hasIllegalImages
}

concerns contains flag if {
	has_illegal_images
	flag := {
		"id": "ovirt.disk.illegal_images.detected",
		"category": "Critical",
		"label": "Illegal disk images detected",
		"assessment": "The VM has one or more snapshots with disks in ILLEGAL state, which is not currently supported by OpenShift Virtualization. The VM disk transfer is likely to fail.",
	}
}
