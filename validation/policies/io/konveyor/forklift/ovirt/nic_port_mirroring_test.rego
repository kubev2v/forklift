package io.konveyor.forklift.ovirt

# NIC tests contain the attributes to match each and all of the NIC rules

test_without_port_mirroring {
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

test_with_port_mirroring {
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
            },
            {
                "id" : "656e7031-7330-3030-3a31-613a34613a32",
                "interface": "e1000",
                "plugged": true,
                "profile": {
                    "portMirroring": true,
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