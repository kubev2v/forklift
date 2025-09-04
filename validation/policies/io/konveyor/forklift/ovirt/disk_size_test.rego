package io.konveyor.forklift.ovirt

import rego.v1

test_invalid_capacity_zero if {
	test_input := {"diskAttachments": [{
		"id": "disk1-id",
		"interface": "sata",
		"disk": {
			"storageType": "image",
			"status": "ok",
			"provisionedSize": 0,
		},
	}]}

	results := concerns with input as test_input
	count(results) == 1
}

test_invalid_capacity_negative if {
	test_input := {"diskAttachments": [{
		"id": "disk2-id",
		"interface": "sata",
		"disk": {
			"storageType": "image",
			"status": "ok",
			"provisionedSize": -1024,
		},
	}]}

	results := concerns with input as test_input
	count(results) == 1
}

test_valid_capacity if {
	test_input := {"diskAttachments": [{
		"id": "disk3-id",
		"interface": "sata",
		"disk": {
			"storageType": "image",
			"status": "ok",
			"provisionedSize": 17179869184,
		},
	}]}

	results := concerns with input as test_input
	count(results) == 0
}
