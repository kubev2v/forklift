package io.konveyor.forklift.ovirt

# NIC tests contain the attributes to match each and all of the NIC rules

test_without_qos {
    mock_vm := {
        "name": "test",
        "nics": [
            {
                "id" : "656e7031-7330-3030-3a31-613a34613a31",
                "interface": "e1000",
                "plugged": true,
                "properties": [],
                "profile": {
                    "portMirroring": false,
                    "networkFilter": "",
                    "qos": ""
                }
            }
        ]
    }
    results := concerns with input as mock_vm
    count(results) == 0
}

test_with_qos {
    mock_vm := {
        "name": "test",
        "nics": [
            {
                "id" : "656e7031-7330-3030-3a31-613a34613a31",
                "interface": "e1000",
                "plugged": true,
                "properties": [],
                "profile": {
                    "portMirroring": false,
                    "networkFilter": "",
                    "qos": ""
                }
            },
            {
                "id" : "656e7031-7330-3030-3a31-613a34613a32",
                "interface": "e1000",
                "plugged": true,
                "properties": [],
                "profile": {
                    "portMirroring": false,
                    "networkFilter": "",
                    "qos": "something"
                }
            }
        ]
    }
    results := concerns with input as mock_vm
    count(results) == 1
}