package io.konveyor.forklift.ovirt

import rego.v1

test_unsupported_el6_64 if {
	mock_vm := {
		"name": "test",
		"osType": "rhel_6x64",
	}
	results = concerns with input as mock_vm
	count(results) == 1
}

test_supported_el7 if {
	mock_vm := {
		"name": "test",
		"osType": "rhel_7x64",
	}
	results = concerns with input as mock_vm
	count(results) == 0
}

test_unsupported_el6 if {
	mock_vm := {
		"name": "test",
		"osType": "rhel_6",
	}
	results = concerns with input as mock_vm
	count(results) == 1
}

test_supported_windows if {
	mock_vm := {
		"name": "test",
		"osType": "windows_2019x64",
	}
	results = concerns with input as mock_vm
	count(results) == 0
}
