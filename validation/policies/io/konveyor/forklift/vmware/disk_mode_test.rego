package io.konveyor.forklift.vmware

test_with_no_disks {
    mock_vm := {
        "name": "test",
        "disks": []
    }
    results := concerns with input as mock_vm
    count(results) == 0
}

test_with_no_independent_disk {
    mock_vm := {
        "name": "test",
        "disks": [
            { "shared": false },
            { "shared": false, "mode": "dependent" }
        ]
    }
    results := concerns with input as mock_vm
    count(results) == 0
}

test_with_independent_persistent_disk {
    mock_vm := {
        "name": "test",
        "disks": [
            { "shared": false },
            { "shared": false, "mode": "dependent" },
            { "shared": false, "mode": "independent_persistent" }
        ]
    }
    results := concerns with input as mock_vm
    count(results) == 1
}

test_with_independent_nonpersistent_disk {
    mock_vm := {
        "name": "test",
        "disks": [
            { "shared": false, "mode": "independent_nonpersistent" }
        ]
    }
    results := concerns with input as mock_vm
    count(results) == 1
}

