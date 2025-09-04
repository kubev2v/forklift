package io.konveyor.forklift.ovirt

import rego.v1

default has_usb_enabled := false

has_usb_enabled := value if {
	value := input.usbEnabled
}

concerns contains flag if {
	has_usb_enabled
	flag := {
		"id": "ovirt.usb.enabled",
		"category": "Warning",
		"label": "USB support enabled",
		"assessment": "The VM has USB support enabled, but USB device attachment is not currently supported by OpenShift Virtualization.",
	}
}
