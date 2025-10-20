package io.konveyor.forklift.vmware

import rego.v1

has_usb_controller if {
	some i
	input.devices[i].kind == "VirtualUSBController"
}

concerns contains flag if {
	has_usb_controller
	flag := {
		"id": "vmware.usb_controller.detected",
		"category": "Warning",
		"label": "USB controller detected",
		"assessment": "USB controllers are not currently supported by Migration Toolkit for Virtualization. The VM can be migrated but the devices attached to the USB controller will not be migrated. Administrators can configure this after migration.",
	}
}
