package io.konveyor.forklift.ovirt

import rego.v1

test_without_shared_disk if {
	mock_vm := {
		"name": "test",
		"diskAttachments": [{"disk": {"shared": false}}],
	}
	results := concerns with input as mock_vm
	count(results) == 0
}

test_with_shared_disk if {
	mock_vm := {
		"name": "test",
		"diskAttachments": [
			{"disk": {"shared": false}},
			{"disk": {"shared": true}},
		],
	}
	results := concerns with input as mock_vm
	count(results) == 1
}
