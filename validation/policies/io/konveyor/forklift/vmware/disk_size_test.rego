package io.konveyor.forklift.vmware
import future.keywords.in


test_invalid_capacity_zero {
    input := {
        "disks": [
            {
                "file": "disk1.vmdk",
                "capacity": 0
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
                "file": "disk2.vmdk",
                "capacity": -1024
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
                "file": "disk3.vmdk",
                "capacity": 17179869184
            }
        ]
    }

    results := concerns with input as input
    count(results) == 0
}
