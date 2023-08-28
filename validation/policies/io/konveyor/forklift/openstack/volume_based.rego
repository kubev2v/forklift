package io.konveyor.forklift.openstack

import future.keywords.if

default not_volume_based = false

not_volume_based if {
	input.imageID != ""
}

concerns[flag] {
	not_volume_based
	flag := {
		"category": "Critical",
		"label": "Unsupported image format detected",
		"assessment": "The VM is not volume based, currently only volume based VMs can be imported.",
	}
}
