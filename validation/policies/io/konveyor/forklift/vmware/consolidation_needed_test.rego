package io.konveyor.forklift.vmware

import rego.v1

test_consolidation_needed if {
	mock_vm := {
		"name": "test",
		"consolidationNeeded": true
	}

	results := concerns with input as mock_vm
	count(results) == 1
}

test_consolidation_not_needed if {
	mock_vm := {
		"name": "test",
		"consolidationNeeded": false
	}

	results := concerns with input as mock_vm
	count(results) == 0
}

