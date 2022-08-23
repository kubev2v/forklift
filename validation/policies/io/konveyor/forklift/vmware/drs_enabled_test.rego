package io.konveyor.forklift.vmware

test_without_drs_enabled {
    mock_vm := {
        "name": "test",
        "host": {
            "name": "test_host",
            "cluster": {
                "name": "test_cluster",
                "drsEnabled": false
                
            }
        }
    }
    results := concerns with input as mock_vm
    count(results) == 0
}

test_with_drs_enabled {
    mock_vm := {
        "name": "test",
        "host": {
            "name": "test_host",
            "cluster": {
                "name": "test_cluster",
                "drsEnabled": true
                
            }
        }
    }
    results := concerns with input as mock_vm
    count(results) == 1
}
