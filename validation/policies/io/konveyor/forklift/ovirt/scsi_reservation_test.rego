package io.konveyor.forklift.ovirt

test_without_scsi_reservation {
    mock_vm := {
        "name": "test",
        "diskAttachments": [
            { "scsiReservation": false }
        ]
    }
    results := concerns with input as mock_vm
    count(results) == 0
}

test_with_scsi_reservation {
    mock_vm := {
        "name": "test",
        "diskAttachments": [
            { "scsiReservation": false },
            { "scsiReservation": true }
        ]
    }
    results := concerns with input as mock_vm
    count(results) == 1
}