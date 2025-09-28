package io.konveyor.forklift.ova

import rego.v1

test_with_unsupported_source if {
	mock_vm := {"name": "test", "ovaSource": "Unknown"}
	results = concerns with input as mock_vm
	count(results) == 1
}

test_with_supported_source if {
	mock_vm := {"name": "test", "ovaSource": "VMware"}
	results = concerns with input as mock_vm
	count(results) == 0
}
