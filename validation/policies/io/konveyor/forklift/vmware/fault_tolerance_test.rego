package io.konveyor.forklift.vmware

import rego.v1

test_with_fault_tolerance_disabled if {
	mock_vm := {"name": "test", "faultToleranceEnabled": false}
	results := concerns with input as mock_vm
	count(results) == 0
}

test_with_fault_tolerance_enabled if {
	mock_vm := {"name": "test", "faultToleranceEnabled": true}
	results := concerns with input as mock_vm
	count(results) == 1
}
