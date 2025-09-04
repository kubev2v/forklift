package io.konveyor.forklift.ovirt

import rego.v1

test_without_scsi_reservation if {
	mock_vm := {
		"name": "test",
		"diskAttachments": [{"scsiReservation": false}],
	}
	results := concerns with input as mock_vm
	count(results) == 0
}

test_with_scsi_reservation if {
	mock_vm := {
		"name": "test",
		"diskAttachments": [
			{"scsiReservation": false},
			{"scsiReservation": true},
		],
	}
	results := concerns with input as mock_vm
	count(results) == 1
}
