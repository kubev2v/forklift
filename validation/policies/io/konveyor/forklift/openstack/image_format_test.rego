package io.konveyor.forklift.openstack

test_with_valid_image_format {
	mock_vm := {
		"name": "test",
		"image": {
			"id": "b749c132-bb97-4145-b86e-a1751cf75e21",
			"disk_format": "qcow2",
		},
	}
	results := concerns with input as mock_vm
	count(results) == 0
}

test_with_invalid_storage_type {
	mock_vm := {
		"name": "test",
		"image": {
			"id": "b749c132-bb97-4145-b86e-a1751cf75e21",
			"disk_format": "ami",
		},
	}
	results := concerns with input as mock_vm
	count(results) == 1
}
