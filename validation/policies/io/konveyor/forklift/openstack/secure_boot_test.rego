package io.konveyor.forklift.openstack

import rego.v1

test_with_flavor_secure_boot if {
	mock_vm := {
		"name": "test",
		"flavor": {"extraSpecs": {"os:secure_boot": "required"}},
	}
	results := concerns with input as mock_vm
	count(results) == 1
}

test_with_image_secure_boot if {
	mock_vm := {
		"name": "test",
		"image": {"properties": {"os_secure_boot": "required"}},
	}
	results := concerns with input as mock_vm
	count(results) == 1
}

test_with_optional_secure_boot if {
	mock_vm := {
		"name": "test",
		"image": {"properties": {"os_secure_boot": "optional"}},
	}
	results := concerns with input as mock_vm
	count(results) == 0
}

test_with_disabled_secure_boot if {
	mock_vm := {
		"name": "test",
		"image": {"properties": {"os_secure_boot": "disabled"}},
	}
	results := concerns with input as mock_vm
	count(results) == 0
}

test_without_secure_boot if {
	mock_vm := {"name": "test"}
	results := concerns with input as mock_vm
	count(results) == 0
}
