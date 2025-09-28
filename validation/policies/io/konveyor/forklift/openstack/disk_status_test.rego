package io.konveyor.forklift.openstack

import rego.v1

test_with_valid_disk_status if {
	mock_vm := {
		"name": "test",
		"volumes": [
			{
				"id": "b749c132-bb97-4145-b86e-a1751cf75e21",
				"name": "",
				"status": "in-use",
				"attachments": [{"AttachmentID": "1"}],
			},
			{
				"id": "42d979c7-653c-4dd9-8a51-2f734b250b4d",
				"name": "",
				"status": "available",
				"attachments": [{"AttachmentID": "1"}],
			},
		],
	}
	results := concerns with input as mock_vm
	count(results) == 0
}

test_with_one_invalid_disk_status if {
	mock_vm := {
		"name": "test",
		"volumes": [
			{
				"id": "b749c132-bb97-4145-b86e-a1751cf75e21",
				"name": "",
				"status": "error",
				"attachments": [{"AttachmentID": "1"}],
			},
			{
				"id": "42d979c7-653c-4dd9-8a51-2f734b250b4d",
				"name": "",
				"status": "available",
				"attachments": [{"AttachmentID": "1"}],
			},
		],
	}
	results := concerns with input as mock_vm
	count(results) == 1
}

test_with_two_invalid_disk_status if {
	mock_vm := {
		"name": "test",
		"volumes": [
			{
				"id": "b749c132-bb97-4145-b86e-a1751cf75e21",
				"name": "",
				"status": "error",
				"attachments": [{"AttachmentID": "1"}],
			},
			{
				"id": "42d979c7-653c-4dd9-8a51-2f734b250b4d",
				"name": "",
				"status": "creating",
				"attachments": [{"AttachmentID": "1"}],
			},
		],
	}
	results := concerns with input as mock_vm
	count(results) == 1
}
