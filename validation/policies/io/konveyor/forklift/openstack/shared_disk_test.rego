package io.konveyor.forklift.openstack

test_with_no_volumes {
	mock_vm := {
		"name": "test",
		"volumes": [],
	}
	results := concerns with input as mock_vm
	count(results) == 0
}

test_without_shared_disk {
	mock_vm := {
		"name": "test",
		"volumes": [
			{"id": "1", "status": "in-use", "attachments": [{"AttachmentID": "1"}]},
			{"id": "2", "status": "in-use", "attachments": [{"AttachmentID": "1"}]},
			{"id": "3", "status": "in-use", "attachments": [{"AttachmentID": "1"}]},
			{"id": "4", "status": "in-use", "attachments": []},
			{"id": "5", "status": "in-use" },
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
			{"id": "4", "status": "in-use", "attachments": []},
			{"id": "5", "status": "in-use" },
		],
	}
	results := concerns with input as mock_vm
	count(results) == 1
}
