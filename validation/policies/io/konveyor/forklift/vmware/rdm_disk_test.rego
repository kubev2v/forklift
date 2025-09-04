package io.konveyor.forklift.vmware

import rego.v1

test_with_no_disks if {
	mock_vm := {
		"name": "test",
		"disks": [],
	}
	results := concerns with input as mock_vm
	count(results) == 0
}

test_with_no_shareable_disk if {
	mock_vm := {
		"name": "test",
		"disks": [{"rdm": false}],
	}
	results := concerns with input as mock_vm
	count(results) == 0
}

test_with_shareable_disk if {
	mock_vm := {
		"name": "test",
		"disks": [
			{"rdm": false},
			{"rdm": true},
			{"rdm": false},
		],
	}
	results := concerns with input as mock_vm
	count(results) == 1
}
