package io.konveyor.forklift.openstack

import rego.v1

test_invalid_size_zero if {
	test_input := {"volumes": [{
		"id": "volume1-id",
		"name": "volume1",
		"size": 0,
		"status": "available",
	}]}

	results := concerns with input as test_input
	count(results) == 1
}

test_invalid_size_negative if {
	test_input := {"volumes": [{
		"id": "volume2-id",
		"name": "volume2",
		"size": -10,
		"status": "available",
	}]}

	results := concerns with input as test_input
	count(results) == 1
}

test_valid_size if {
	test_input := {"volumes": [{
		"id": "volume3-id",
		"name": "volume3",
		"size": 20,
		"status": "available",
	}]}

	results := concerns with input as test_input
	count(results) == 0
}
