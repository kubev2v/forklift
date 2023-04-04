package io.konveyor.forklift.openstack

test_without_shared_disk {
	mock_vm := {
		"name": "test",
		"volumes": [
			{"id": "1", "status": "in-use", "attachments": [{"AttachmentID": "1"}]},
			{"id": "2", "status": "in-use", "attachments": [{"AttachmentID": "1"}]},
			{"id": "3", "status": "in-use", "attachments": [{"AttachmentID": "1"}]},
		],
	}
	results := concerns with input as mock_vm
	count(results) == 0
}

test_with_shared_disk {
	mock_vm := {
		"name": "test",
		"volumes": [
			{"id": "1", "status": "in-use", "attachments": [{"AttachmentID": "1"}, {"AttachmentID": "2"}]},
			{"id": "2", "status": "in-use", "attachments": [{"AttachmentID": "1"}]},
			{"id": "3", "status": "in-use", "attachments": [{"AttachmentID": "1"}]},
		],
	}
	results := concerns with input as mock_vm
	count(results) == 1
}
