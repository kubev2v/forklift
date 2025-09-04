package io.konveyor.forklift.openstack

import rego.v1

test_without_numa if {
	mock_vm := {"name": "test", "flavor": {"extraSpecs": {}}}
	results = concerns with input as mock_vm
	count(results) == 0
}

test_with_pci_numa_affinity if {
	mock_vm := {"name": "test", "flavor": {"extraSpecs": {"hw:pci_numa_affinity_policy": "required"}}}
	results = concerns with input as mock_vm
	count(results) == 1
}

test_with_numa_nodes if {
	mock_vm := {"name": "test", "flavor": {"extraSpecs": {"hw:numa_nodes": "2"}}}
	results = concerns with input as mock_vm
	count(results) == 1
}

test_with_all_numa if {
	mock_vm := {"name": "test", "flavor": {"extraSpecs": {"hw:numa_nodes": "2", "hw:pci_numa_affinity_policy": "required"}}}
	results = concerns with input as mock_vm
	count(results) == 1
}
