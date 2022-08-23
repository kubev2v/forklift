package io.konveyor.forklift.ovirt

# NIC tests contain the attributes to match each and all of the NIC rules

test_without_network_filter {
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

test_with_network_filter {
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
            },
            {
                "id" : "656e7031-7330-3030-3a31-613a34613a31",
                "interface": "rtl8139",
                "plugged": true,
                "profile": {
                    "portMirroring": false,
                    "networkFilter": "343f43d2-23eb-11e8-a056-00163e18b6f7",
                    "qos": "",
                    "properties": []
                }
            }
        ]
    }
    results := concerns with input as mock_vm
    count(results) == 1
}