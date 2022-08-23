package io.konveyor.forklift.ovirt

test_with_no_online_snapshot {
    mock_vm := {
        "name": "test",
        "snapshots": [
            {
                "id": "8a678302-003c-442f-a86d-8f6eb874ed2d",
                "description": "Active VM",
                "type": "active",
                "persistMemory": false
            },
        ]
    }
    results := concerns with input as mock_vm
    count(results) == 0
}

test_with_online_snapshot {
    mock_vm := {
        "name": "test",
        "snapshots": [
            {
                "id": "8a678302-003c-442f-a86d-8f6eb874ed2d",
                "description": "Active VM",
                "type": "active",
                "persistMemory": false
            },
            {
                "id": "26950c1d-01e6-4c71-9eae-02842f341f1b",
                "description": "online",
                "type": "regular",
                "persistMemory": true
            },
            {
                "id": "2ef746c2-7238-4e74-b45b-bfd2ad6447e2",
                "description": "Next Run configuration snapshot",
                "type": "",
                "persistMemory": false
            }
        ]
    }
    results := concerns with input as mock_vm
    count(results) == 1
}
