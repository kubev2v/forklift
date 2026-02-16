package io.konveyor.forklift.hyperv

import rego.v1

test_has_checkpoint if {
	mock_vm := {
		"name": "test-vm",
		"powerState": "On",
		"hasCheckpoint": true,
		"disks": [{"name": "disk-0", "capacity": 1000}],
	}
	results := concerns with input as mock_vm
	some result in results
	result.id == "hyperv.checkpoint.detected"
}

test_no_checkpoint if {
	mock_vm := {
		"name": "test-vm",
		"powerState": "On",
		"hasCheckpoint": false,
		"disks": [{"name": "disk-0", "capacity": 1000}],
	}
	results := concerns with input as mock_vm
	not any_checkpoint_concern(results)
}

test_checkpoint_not_set if {
	mock_vm := {
		"name": "test-vm",
		"powerState": "On",
		"disks": [{"name": "disk-0", "capacity": 1000}],
	}
	results := concerns with input as mock_vm
	not any_checkpoint_concern(results)
}

any_checkpoint_concern(results) if {
	some result in results
	result.id == "hyperv.checkpoint.detected"
}
