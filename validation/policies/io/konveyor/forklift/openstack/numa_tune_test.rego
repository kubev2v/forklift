package io.konveyor.forklift.openstack

test_without_numa {
	mock_vm := {"name": "test", "flavor": {"extraSpecs": {}}}
	results = concerns with input as mock_vm
	count(results) == 0
}

test_with_pci_numa_affinity {
	mock_vm := {"name": "test", "flavor": {"extraSpecs": {"hw:pci_numa_affinity_policy": "required"}}}
	results = concerns with input as mock_vm
	count(results) == 1
}

test_with_numa_nodes {
	mock_vm := {"name": "test", "flavor": {"extraSpecs": {"hw:numa_nodes": "2"}}}
	results = concerns with input as mock_vm
	count(results) == 1
}

test_with_all_numa {
	mock_vm := {"name": "test", "flavor": {"extraSpecs": {"hw:numa_nodes": "2", "hw:pci_numa_affinity_policy": "required"}}}
	results = concerns with input as mock_vm
	count(results) == 1
}
