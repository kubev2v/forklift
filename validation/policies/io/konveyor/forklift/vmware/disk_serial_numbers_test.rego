package io.konveyor.forklift.vmware

test_with_uuid_enabled_scsi {
    mock_vm := {
        "name": "test",
	"diskEnableUuid": true,
	"disks": [{ "bus": "scsi" }]
    }
    results := concerns with input as mock_vm
    count(results) == 1
}

test_with_uuid_enabled_sata {
    mock_vm := {
        "name": "test",
	"diskEnableUuid": true,
	"disks": [{ "bus": "sata" }]
    }
    results := concerns with input as mock_vm
    count(results) == 0
}

test_with_uuid_disabled {
    mock_vm := {
        "name": "test",
	"diskEnableUuid": false
    }
    results := concerns with input as mock_vm
    count(results) == 0
}
