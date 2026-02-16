package io.konveyor.forklift.hyperv

import rego.v1

any_rct_concern(results) if {
	some result in results
	result.id == "hyperv.resilient_change_tracking.disabled"
}

# Test with RCT enabled on all disks - no warning
test_no_warning_when_rct_enabled if {
	result := concerns with input as {
		"name": "test-vm",
		"disks": [
			{"id": "disk-0", "rctEnabled": true},
			{"id": "disk-1", "rctEnabled": true},
		],
	}
	not any_rct_concern(result)
}

# Test with RCT disabled on one disk - warning expected
test_warning_when_rct_disabled if {
	result := concerns with input as {
		"name": "test-vm",
		"disks": [
			{"id": "disk-0", "rctEnabled": true},
			{"id": "disk-1", "rctEnabled": false},
		],
	}
	some r in result
	r.id == "hyperv.resilient_change_tracking.disabled"
}

# Test with RCT disabled on all disks - warning expected
test_warning_when_all_rct_disabled if {
	result := concerns with input as {
		"name": "test-vm",
		"disks": [
			{"id": "disk-0", "rctEnabled": false},
			{"id": "disk-1", "rctEnabled": false},
		],
	}
	some r in result
	r.id == "hyperv.resilient_change_tracking.disabled"
}

# Test with no disks - no warning (edge case)
test_no_warning_when_no_disks if {
	result := concerns with input as {
		"name": "test-vm",
		"disks": [],
	}
	not any_rct_concern(result)
}

# Test with single disk RCT disabled - warning expected
test_warning_single_disk_rct_disabled if {
	result := concerns with input as {
		"name": "test-vm",
		"disks": [{"id": "disk-0", "rctEnabled": false}],
	}
	some r in result
	r.id == "hyperv.resilient_change_tracking.disabled"
	r.category == "Warning"
}
