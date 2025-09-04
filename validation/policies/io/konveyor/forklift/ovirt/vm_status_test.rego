package io.konveyor.forklift.ovirt

import rego.v1

test_with_first_valid_status if {
	mock_vm := {
		"name": "test",
		"status": "up",
	}
	results := concerns with input as mock_vm
	count(results) == 0
}

test_with_second_valid_status if {
	mock_vm := {
		"name": "test",
		"status": "down",
	}
	results := concerns with input as mock_vm
	count(results) == 0
}

test_with_invalid_status if {
	mock_vm := {
		"name": "test",
		"status": "paused",
	}
	results := concerns with input as mock_vm
	count(results) == 1
}
