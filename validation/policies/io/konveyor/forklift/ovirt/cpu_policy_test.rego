package io.konveyor.forklift.ovirt

import rego.v1

test_with_none if {
	mock_vm := {
		"name": "test",
		"cpuPinningPolicy": "none",
	}
	results := concerns with input as mock_vm
	count(results) == 0
}

test_with_dedicated if {
	mock_vm := {
		"name": "test",
		"cpuPinningPolicy": "dedicated",
	}
	results := concerns with input as mock_vm
	count(results) == 0
}

test_with_manual if {
	mock_vm := {
		"name": "test",
		"cpuPinningPolicy": "manual",
		"cpuAffinity": [0, 2],
	}
	results := concerns with input as mock_vm
	count(results) == 1
}

test_with_resize_and_pin_numa if {
	mock_vm := {
		"name": "test",
		"cpuPinningPolicy": "resize_and_pin_numa",
	}
	results := concerns with input as mock_vm
	count(results) == 1
}

test_with_isolate_threads if {
	mock_vm := {
		"name": "test",
		"cpuPinningPolicy": "isolate_threads",
	}
	results := concerns with input as mock_vm
	count(results) == 1
}
