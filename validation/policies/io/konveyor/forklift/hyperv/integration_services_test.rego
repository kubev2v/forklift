package io.konveyor.forklift.hyperv

import rego.v1

test_powered_on_no_guest_os if {
	mock_vm := {
		"name": "test-vm",
		"powerState": "On",
		"guestOS": "",
		"disks": [{"name": "disk-0", "capacity": 1000}],
	}
	results := concerns with input as mock_vm
	some result in results
	result.id == "hyperv.integration_services.not_detected"
}

test_powered_on_with_guest_os if {
	mock_vm := {
		"name": "test-vm",
		"powerState": "On",
		"guestOS": "Windows Server 2019",
		"disks": [{"name": "disk-0", "capacity": 1000}],
	}
	results := concerns with input as mock_vm
	not any_integration_services_concern(results)
}

test_powered_off_no_guest_os if {
	mock_vm := {
		"name": "test-vm",
		"powerState": "Off",
		"guestOS": "",
		"disks": [{"name": "disk-0", "capacity": 1000}],
	}
	results := concerns with input as mock_vm
	not any_integration_services_concern(results)
}

any_integration_services_concern(results) if {
	some result in results
	result.id == "hyperv.integration_services.not_detected"
}
