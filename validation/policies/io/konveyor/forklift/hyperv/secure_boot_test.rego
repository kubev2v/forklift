package io.konveyor.forklift.hyperv

import rego.v1

test_secure_boot_enabled if {
	mock_vm := {
		"name": "test-vm",
		"powerState": "On",
		"secureBoot": true,
		"disks": [{"name": "disk-0", "capacity": 1000}],
	}
	results := concerns with input as mock_vm
	some result in results
	result.id == "hyperv.secure_boot.detected"
}

test_secure_boot_disabled if {
	mock_vm := {
		"name": "test-vm",
		"powerState": "On",
		"secureBoot": false,
		"disks": [{"name": "disk-0", "capacity": 1000}],
	}
	results := concerns with input as mock_vm
	not any_secure_boot_concern(results)
}

test_secure_boot_not_set if {
	mock_vm := {
		"name": "test-vm",
		"powerState": "On",
		"disks": [{"name": "disk-0", "capacity": 1000}],
	}
	results := concerns with input as mock_vm
	not any_secure_boot_concern(results)
}

any_secure_boot_concern(results) if {
	some result in results
	result.id == "hyperv.secure_boot.detected"
}
