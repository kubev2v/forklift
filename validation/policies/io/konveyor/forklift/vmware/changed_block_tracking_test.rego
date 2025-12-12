package io.konveyor.forklift.vmware

import rego.v1

test_with_changed_block_tracking_enabled if {
	mock_vm := {"name": "test", "changeTrackingEnabled": true}
	results := concerns with input as mock_vm
	count(results) == 0
}

test_with_changed_block_tracking_disabled if {
	mock_vm := {"name": "test", "changeTrackingEnabled": false}
	results := concerns with input as mock_vm
	count(results) == 1
}

test_with_changed_block_tracking_enabled_with_snapshots if {
	mock_vm := {
		"name": "test", 
		"changeTrackingEnabled": true,
		"snapshot": {"id": "snapshot-123"}
	}
	results := concerns with input as mock_vm
	count(results) == 0
}

test_with_changed_block_tracking_disabled_with_snapshots if {
	mock_vm := {
		"name": "test", 
		"changeTrackingEnabled": false,
		"snapshot": {"id": "snapshot-123"}
	}
	results := concerns with input as mock_vm
	count(results) == 1
}
