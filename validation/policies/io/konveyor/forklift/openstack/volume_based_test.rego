package io.konveyor.forklift.openstack

test_image_based_vm {
	mock_vm := {
		"name": "test",
		"imageID": "b749c132-bb97-4145-b86e-a1751cf75e21",
		"image": {
			"id": "b749c132-bb97-4145-b86e-a1751cf75e21",
		},
	}
	results := concerns with input as mock_vm
	count(results) == 1
}

test_volume_based_vm {
	mock_vm := {
		"name": "test",
		"image": {
			"id": "b749c132-bb97-4145-b86e-a1751cf75e21",
		},
		"volumes": []
	}
	results := concerns with input as mock_vm
	count(results) == 0
}
