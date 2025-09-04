package io.konveyor.forklift.openstack

import rego.v1

test_with_first_valid_status if {
	mock_vm := {
		"name": "test",
		"status": "ACTIVE",
	}
	results := concerns with input as mock_vm
	count(results) == 0
}

test_with_second_valid_status if {
	mock_vm := {
		"name": "test",
		"status": "SHUTOFF",
	}
	results := concerns with input as mock_vm
	count(results) == 0
}

test_with_invalid_status if {
	mock_vm := {
		"name": "test",
		"status": "PAUSED",
	}
	results := concerns with input as mock_vm
	count(results) == 1
}
