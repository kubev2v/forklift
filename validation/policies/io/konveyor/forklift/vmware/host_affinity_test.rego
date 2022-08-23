package io.konveyor.forklift.vmware

test_without_host_affinity_vms {
    mock_vm := {
        "name": "test",
        "id": "vm-123",
        "host": {
            "name": "test_host",
            "cluster": {
                "name": "test_cluster",
                "hostAffinityVms": []
            }
        }
    }
    results := concerns with input as mock_vm
    count(results) == 0
}

test_with_other_host_affinity_vms {
    mock_vm := {
        "name": "test",
        "id": "vm-123",
        "host": {
            "name": "test_host",
            "cluster": {
                "name": "test_cluster",
                "hostAffinityVms": [
                {
                    "kind": "VM",
                    "id": "vm-2050"
                },
                {
                    "kind": "VM",
                    "id": "vm-2696"
                }
                ]
            }
        }
    }
    results := concerns with input as mock_vm
    count(results) == 0
}

test_with_host_affinity_vm {
    mock_vm := {
        "name": "test",
        "id": "vm-123",
        "host": {
            "name": "test_host",
            "cluster": {
                "name": "test_cluster",
                "hostAffinityVms": [
                {
                    "kind": "VM",
                    "id": "vm-123"
                }
                ]
            }
        }
    }
    results := concerns with input as mock_vm
    count(results) == 1
}
