package io.konveyor.forklift.ova
import future.keywords.in

test_invalid_capacity_zero {
    input := {
        "disks": [
            {
                "filePath": "disk1.vmdk",
                "capacity": 0,
                "format": "vmdk"
            }
        ]
    }

    results := concerns with input as input
    count(results) == 1
}

test_invalid_capacity_negative {
    input := {
        "disks": [
            {
                "filePath": "disk2.vmdk",
                "capacity": -1024,
                "format": "vmdk"
            }
        ]
    }

    results := concerns with input as input
    count(results) == 1
}

test_valid_capacity {
    input := {
        "disks": [
            {
                "filePath": "disk3.vmdk", 
                "capacity": 17179869184,
                "format": "vmdk"
            }
        ]
    }

    results := concerns with input as input
    count(results) == 0
} 