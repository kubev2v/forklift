package io.konveyor.forklift.ova

import rego.v1

test_valid_vm_name if {
	mock_vm := {"name": "test"}
	results := concerns with input as mock_vm
	count(results) == 0
}

test_vm_name_too_long if {
	mock_vm := {"name": "my-vm-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"}
	results := concerns with input as mock_vm
	count(results) == 1
}

test_vm_name_invalid_char_underscore if {
	mock_vm := {"name": "my_vm"}
	results := concerns with input as mock_vm
	count(results) == 1
}

test_vm_name_invalid_char_slash if {
	mock_vm := {"name": "my/vm"}
	results := concerns with input as mock_vm
	count(results) == 1
}
