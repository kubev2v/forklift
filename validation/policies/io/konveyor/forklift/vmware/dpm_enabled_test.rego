package io.konveyor.forklift.vmware

import rego.v1

test_without_dpm_enabled if {
	mock_vm := {
		"name": "test",
		"host": {
			"name": "test_host",
			"cluster": {
				"name": "test_cluster",
				"dpmEnabled": false,
			},
		},
	}
	results := concerns with input as mock_vm
	count(results) == 0
}

test_with_dpm_enabled if {
	mock_vm := {
		"name": "test",
		"host": {
			"name": "test_host",
			"cluster": {
				"name": "test_cluster",
				"dpmEnabled": true,
			},
		},
	}
	results := concerns with input as mock_vm
	count(results) == 1
}
