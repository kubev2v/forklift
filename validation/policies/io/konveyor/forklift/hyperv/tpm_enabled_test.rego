package io.konveyor.forklift.hyperv

import rego.v1

test_tpm_enabled if {
	mock_vm := {
		"name": "test-vm",
		"powerState": "On",
		"tpmEnabled": true,
		"disks": [{"name": "disk-0", "capacity": 1000}],
	}
	results := concerns with input as mock_vm
	some result in results
	result.id == "hyperv.tpm.detected"
}

test_tpm_disabled if {
	mock_vm := {
		"name": "test-vm",
		"powerState": "On",
		"tpmEnabled": false,
		"disks": [{"name": "disk-0", "capacity": 1000}],
	}
	results := concerns with input as mock_vm
	not any_tpm_concern(results)
}

test_tpm_not_set if {
	mock_vm := {
		"name": "test-vm",
		"powerState": "On",
		"disks": [{"name": "disk-0", "capacity": 1000}],
	}
	results := concerns with input as mock_vm
	not any_tpm_concern(results)
}

any_tpm_concern(results) if {
	some result in results
	result.id == "hyperv.tpm.detected"
}
