package io.konveyor.forklift.openstack

test_volume_based_vm {
	mock_vm := {
		"name": "test",
		"status": "ACTIVE",
	}
	results := concerns with input as mock_vm
	count(results) == 0
}

test_image_based_vm {
	mock_vm := {
		"name": "test",
		"status": "ACTIVE",
		"imageID": "1",
	}
	results := concerns with input as mock_vm
	count(results) == 1
}
