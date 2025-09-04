package io.konveyor.forklift.ovirt

import rego.v1

test_without_ksm_enabled if {
	mock_vm := {
		"name": "test",
		"cluster": {"ksmEnabled": false},
	}
	results = concerns with input as mock_vm
	count(results) == 0
}

test_with_ksm_enabled if {
	mock_vm := {
		"name": "test",
		"cluster": {"ksmEnabled": true},
	}
	results = concerns with input as mock_vm
	count(results) == 1
}
