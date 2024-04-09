package io.konveyor.forklift.vmware

test_with_no_disks {
    mock_vm := {
        "name": "test",
        "disks": []
    }
    results := concerns with input as mock_vm
    count(results) == 0
}

test_with_valid_disk {
    mock_vm := {
        "name": "test",
        "disks": [
            { "datastore": { "kind": "datastore", "id": "datastore-1" } }
        ]
    }
    results := concerns with input as mock_vm
    count(results) == 0
}

test_with_invalid_disk {
    mock_vm := {
        "name": "test",
        "disks": [
            { "datastore": { "kind": "datastore", "id": "datastore-1" } },
            { "datastore": { "kind": "", "id": "" } },
        ]
    }
    results := concerns with input as mock_vm
    count(results) == 1
}
