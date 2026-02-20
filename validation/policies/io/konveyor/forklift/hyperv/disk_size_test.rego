package io.konveyor.forklift.hyperv

import rego.v1

test_valid_disk_capacity if {
	mock_vm := {
		"name": "test-vm",
		"guestNetworks": [{"ip": "10.0.0.1"}],
		"disks": [{"name": "disk-0", "capacity": 10737418240}],
	}
	results := concerns with input as mock_vm
	not any_disk_concern(results)
}

any_disk_concern(results) if {
	some result in results
	startswith(result.id, "hyperv.disk.capacity")
}

test_zero_disk_capacity if {
	mock_vm := {
		"name": "test-vm",
		"guestNetworks": [{"ip": "10.0.0.1"}],
		"disks": [{"name": "disk-0", "capacity": 0}],
	}
	results := concerns with input as mock_vm
	some result in results
	startswith(result.id, "hyperv.disk.capacity")
}

test_negative_disk_capacity if {
	mock_vm := {
		"name": "test-vm",
		"guestNetworks": [{"ip": "10.0.0.1"}],
		"disks": [{"name": "disk-0", "capacity": -1}],
	}
	results := concerns with input as mock_vm
	some result in results
	startswith(result.id, "hyperv.disk.capacity")
}
