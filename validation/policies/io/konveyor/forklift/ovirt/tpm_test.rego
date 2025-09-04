package io.konveyor.forklift.ovirt

import rego.v1

test_without_tpm_enabled if {
	mock_vm := {
		"name": "test",
		"osType": "rhel_9x64",
	}
	results = concerns with input as mock_vm
	count(results) == 0
}

test_with_tpm_enabled_w11 if {
	mock_vm := {
		"name": "test",
		"osType": "windows_11",
	}
	results = concerns with input as mock_vm
	count(results) == 1
}

test_with_tpm_enabled_w2k22 if {
	mock_vm := {
		"name": "test",
		"osType": "windows_2022",
	}
	results = concerns with input as mock_vm
	count(results) == 1
}
