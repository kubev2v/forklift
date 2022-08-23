package io.konveyor.forklift.ovirt

# NIC tests contain the attributes to match each and all of the NIC rules

test_with_no_pci_passthrough {
    mock_vm := {
        "name": "test",
        "nics": [
            {
                "id" : "656e7031-7330-3030-3a31-613a34613a31",
                "interface": "e1000",
                "plugged": true,
                "profile": {
                    "portMirroring": false,
                    "networkFilter": "",
                    "qos": "",
                    "properties": []
                }
            }
        ]
    }
    results := concerns with input as mock_vm
    count(results) == 0
}

test_with_no_pci_passthrough  {
    mock_vm := {
        "name": "test",
        "nics": [
            {
                "id" : "656e7031-7330-3030-3a31-613a34613a31",
                "interface": "pci_passthrough",
                "plugged": true,
                "profile": {
                    "portMirroring": false,
                    "networkFilter": "",
                    "qos": "",
                    "properties": []
                }
            }
        ]
    }
    results := concerns with input as mock_vm
    # count should be 2 as this test also invalidates the nic_interface_type rule
    count(results) == 2
}