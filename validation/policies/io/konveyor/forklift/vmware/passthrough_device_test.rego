package io.konveyor.forklift.vmware

test_with_no_device {
    mock_vm := {
        "name": "test",
        "devices": []
    }
    results := concerns with input as mock_vm
    count(results) == 0
}

test_with_other_xyz_device {
    mock_vm := {
        "name": "test",
        "devices": [
            { "kind": "VirtualXYZEthernetCard" }
        ]
    }
    results := concerns with input as mock_vm
    count(results) == 0
}

test_with_pci_passthrough_device {
    mock_vm := {
        "name": "test",
        "devices": [
            { "kind": "VirtualPCIPassthrough" }
        ]
    }
    results := concerns with input as mock_vm
    count(results) == 1
}
