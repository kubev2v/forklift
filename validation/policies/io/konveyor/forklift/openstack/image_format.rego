package io.konveyor.forklift.openstack

import future.keywords.if

default has_invalid_image_format = false

has_invalid_image_format if {
	not regex.match(`qcow2|raw`, input.image.disk_format)
}

concerns[flag] {
	has_invalid_image_format
	flag := {
		"category": "Critical",
		"label": "Unsupported image format detected",
		"assessment": "The VM image has a format other than 'qcow2' or 'raw', which is not currently supported by OpenShift Virtualization. The VM disk transfer is likely to fail.",
	}
}
