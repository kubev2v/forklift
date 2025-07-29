package io.konveyor.forklift.openstack
import future.keywords.in

test_invalid_size_zero {
    input := {
        "volumes": [
            {
                "id": "volume1-id",
                "name": "volume1",
                "size": 0,
                "status": "available"
            }
        ]
    }

    results := concerns with input as input
    count(results) == 1
}

test_invalid_size_negative {
    input := {
        "volumes": [
            {
                "id": "volume2-id",
                "name": "volume2", 
                "size": -10,
                "status": "available"
            }
        ]
    }

    results := concerns with input as input
    count(results) == 1
}

test_valid_size {
    input := {
        "volumes": [
            {
                "id": "volume3-id",
                "name": "volume3",
                "size": 20,
                "status": "available"
            }
        ]
    }

    results := concerns with input as input
    count(results) == 0
} 