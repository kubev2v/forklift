package io.konveyor.forklift.vmware

test_with_no_snapshot {
    mock_vm := {
        "name": "test",
        "snapshot": {
            "kind": "",
            "id": ""
        },
    }
    results := concerns with input as mock_vm
    count(results) == 0
}

test_with_snapshot {
    mock_vm := {
        "name": "test",
        "snapshot": {
            "kind": "VirtualMachineSnapshot",
            "id": "snapshot-3134"
        },
    }
    results := concerns with input as mock_vm
    count(results) == 1
}
