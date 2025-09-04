package io.konveyor.forklift.ovirt

import rego.v1

test_without_ha_reservation if {
	mock_vm := {
		"name": "test",
		"cluster": {"haReservation": false},
	}
	results = concerns with input as mock_vm
	count(results) == 0
}

test_with_ha_reservation if {
	mock_vm := {
		"name": "test",
		"cluster": {"haReservation": true},
	}
	results = concerns with input as mock_vm
	count(results) == 1
}
