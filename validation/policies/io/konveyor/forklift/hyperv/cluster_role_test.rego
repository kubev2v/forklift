package io.konveyor.forklift.hyperv

import rego.v1

test_cluster_mode_not_registered if {
	mock_vm := {
		"name": "test-vm",
		"powerState": "On",
		"managementType": "cluster",
		"isClusterRole": false,
		"disks": [{"name": "disk-0", "capacity": 1000}],
	}
	results := concerns with input as mock_vm
	some result in results
	result.id == "hyperv.cluster_role.not_registered"
}

test_cluster_mode_registered if {
	mock_vm := {
		"name": "test-vm",
		"powerState": "On",
		"managementType": "cluster",
		"isClusterRole": true,
		"disks": [{"name": "disk-0", "capacity": 1000}],
	}
	results := concerns with input as mock_vm
	not any_cluster_role_concern(results)
}

test_standalone_mode_not_registered if {
	mock_vm := {
		"name": "test-vm",
		"powerState": "On",
		"managementType": "standalone",
		"isClusterRole": false,
		"disks": [{"name": "disk-0", "capacity": 1000}],
	}
	results := concerns with input as mock_vm
	not any_cluster_role_concern(results)
}

test_no_management_type_not_registered if {
	mock_vm := {
		"name": "test-vm",
		"powerState": "On",
		"isClusterRole": false,
		"disks": [{"name": "disk-0", "capacity": 1000}],
	}
	results := concerns with input as mock_vm
	not any_cluster_role_concern(results)
}

any_cluster_role_concern(results) if {
	some result in results
	result.id == "hyperv.cluster_role.not_registered"
}
