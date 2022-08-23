package io.konveyor.forklift.vmware

test_with_no_device {
    mock_vm := {
        "name": "test",
        "devices": []
    }
    results := concerns with input as mock_vm
    count(results) == 0
}

test_with_other_yyy_device {
    mock_vm := {
        "name": "test",
        "devices": [
            { "kind": "VirtualYYYPassthrough" }
        ]
    }
    results := concerns with input as mock_vm
    count(results) == 0
}

test_with_sriov_nic {
    mock_vm := {
        "name": "test",
        "devices": [
            { "kind": "VirtualSriovEthernetCard" }
        ]
    }
    results := concerns with input as mock_vm
    count(results) == 1
}
