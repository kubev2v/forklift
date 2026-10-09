package io.konveyor.forklift.ova

import rego.v1

test_with_unsupported_source if {
	mock_vm := {"name": "test", "exportSource": "Unknown"}
	results := concerns with input as mock_vm
	count(results) == 1
}

test_with_supported_vmware_source if {
	mock_vm := {"name": "test", "exportSource": "VMware"}
	results := concerns with input as mock_vm
	count(results) == 0
}

test_with_supported_nutanix_source if {
	mock_vm := {"name": "test", "exportSource": "Nutanix"}
	results := concerns with input as mock_vm
	count(results) == 0
}

test_with_legacy_ova_source_key if {
	mock_vm := {"name": "test", "ovaSource": "VMware"}
	results := concerns with input as mock_vm
	count(results) == 0
}
