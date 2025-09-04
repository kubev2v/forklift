package io.konveyor.forklift.ovirt

import rego.v1

test_without_usb_enabled if {
	mock_vm := {
		"name": "test",
		"usbEnabled": false,
	}
	results = concerns with input as mock_vm
	count(results) == 0
}

test_with_usb_enabled if {
	mock_vm := {
		"name": "test",
		"usbEnabled": true,
	}
	results = concerns with input as mock_vm
	count(results) == 1
}
