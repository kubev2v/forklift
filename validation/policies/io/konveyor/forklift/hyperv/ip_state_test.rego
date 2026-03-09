package io.konveyor.forklift.hyperv

import rego.v1

test_with_no_guest_networks if {
	mock_vm := {"name": "test-vm", "guestNetworks": [], "disks": [{"Name": "disk-0", "Capacity": 1000}]}
	results := concerns with input as mock_vm
	some result in results
	result.id == "hyperv.vm_missing_ip.detected"
}

test_with_guest_networks if {
	mock_vm := {
		"name": "test-vm",
		"guestNetworks": [{"mac": "00:15:5D:01:DB:01", "ip": "10.0.0.1"}],
		"disks": [{"Name": "disk-0", "Capacity": 1000}],
	}
	results := concerns with input as mock_vm
	not any_ip_concern(results)
}

any_ip_concern(results) if {
	some result in results
	result.id == "hyperv.vm_missing_ip.detected"
}

test_with_null_guest_networks if {
	mock_vm := {"name": "test-vm", "disks": [{"Name": "disk-0", "Capacity": 1000}]}
	results := concerns with input as mock_vm
	some result in results
	result.id == "hyperv.vm_missing_ip.detected"
}
