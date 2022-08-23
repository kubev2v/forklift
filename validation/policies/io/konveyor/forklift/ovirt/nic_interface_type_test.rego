package io.konveyor.forklift.ovirt

# NIC tests contain the attributes to match each and all of the NIC rules

test_with_first_valid_nic_interface_type {
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

test_with_second_valid_nic_interface_type {
    mock_vm := {
        "name": "test",
        "nics": [
            {
                "id" : "656e7031-7330-3030-3a31-613a34613a31",
                "interface": "rtl8139",
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

test_with_third_valid_nic_interface_type {
    mock_vm := {
        "name": "test",
        "nics": [
            {
                "id" : "656e7031-7330-3030-3a31-613a34613a31",
                "interface": "virtio",
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

test_with_invalid_nic_interface_type {
    mock_vm := {
        "name": "test",
        "nics": [
            {
                "id" : "656e7031-7330-3030-3a31-613a34613a31",
                "interface": "broadcom",
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
    count(results) == 1
}