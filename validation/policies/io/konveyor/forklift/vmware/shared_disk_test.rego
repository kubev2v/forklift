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

test_with_no_shared_disk if {
	mock_vm := {
		"name": "test",
		"disks": [{"shared": false}],
	}
	results := concerns with input as mock_vm
	count(results) == 0
}

test_with_shared_disk if {
	mock_vm := {
		"name": "test",
		"disks": [
			{"shared": false},
			{"shared": true},
			{"shared": false},
		],
	}
	results := concerns with input as mock_vm
	count(results) == 1
}
