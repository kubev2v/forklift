package io.konveyor.forklift.vmware

import rego.v1

test_invalid_capacity_zero if {
	test_input := {"disks": [{
		"file": "disk1.vmdk",
		"capacity": 0,
	}]}

	results := concerns with input as test_input
	count(results) == 1
}

test_invalid_capacity_negative if {
	test_input := {"disks": [{
		"file": "disk2.vmdk",
		"capacity": -1024,
	}]}

	results := concerns with input as test_input
	count(results) == 1
}

test_valid_capacity if {
	test_input := {"disks": [{
		"file": "disk3.vmdk",
		"capacity": 17179869184,
	}]}

	results := concerns with input as test_input
	count(results) == 0
}
