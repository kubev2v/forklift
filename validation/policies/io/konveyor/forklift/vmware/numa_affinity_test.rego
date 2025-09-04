package io.konveyor.forklift.vmware

import rego.v1

test_without_cpu_affinity if {
	mock_vm := {"name": "test", "numaNodeAffinity": []}
	results = concerns with input as mock_vm
	count(results) == 0
}

test_with_cpu_affinity if {
	mock_vm := {"name": "test", "numaNodeAffinity": [1, 2]}
	results = concerns with input as mock_vm
	count(results) == 1
}
