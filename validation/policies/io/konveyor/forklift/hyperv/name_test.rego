package io.konveyor.forklift.hyperv

import rego.v1

test_valid_name if {
	mock_vm := {"name": "valid-vm-name", "guestNetworks": [{"ip": "10.0.0.1"}], "disks": [{"Name": "disk-0", "Capacity": 1000}]}
	results := concerns with input as mock_vm
	not any_name_concern(results)
}

any_name_concern(results) if {
	some result in results
	result.id == "hyperv.vm.name.invalid"
}

test_invalid_name_with_spaces if {
	mock_vm := {"name": "VM With Spaces", "guestNetworks": [{"ip": "10.0.0.1"}], "disks": [{"Name": "disk-0", "Capacity": 1000}]}
	results := concerns with input as mock_vm
	some result in results
	result.id == "hyperv.vm.name.invalid"
}

test_invalid_name_with_special_chars if {
	mock_vm := {"name": "vm_with_underscores", "guestNetworks": [{"ip": "10.0.0.1"}], "disks": [{"Name": "disk-0", "Capacity": 1000}]}
	results := concerns with input as mock_vm
	some result in results
	result.id == "hyperv.vm.name.invalid"
}

test_name_too_long if {
	mock_vm := {"name": "this-is-a-very-long-vm-name-that-exceeds-the-maximum-allowed-length-of-63-characters", "guestNetworks": [{"ip": "10.0.0.1"}], "disks": [{"Name": "disk-0", "Capacity": 1000}]}
	results := concerns with input as mock_vm
	some result in results
	result.id == "hyperv.vm.name.invalid"
}
