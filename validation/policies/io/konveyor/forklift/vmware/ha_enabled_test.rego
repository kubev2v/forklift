package io.konveyor.forklift.vmware

test_without_ha_enabled {
    mock_vm := {
        "name": "test",
        "host": {
            "name": "test_host",
            "cluster": {
                "name": "test_cluster",
                "dasEnabled": false
                
            }
        }
    }
    results := concerns with input as mock_vm
    count(results) == 0
}

test_with_ha_enabled {
    mock_vm := {
        "name": "test",
        "host": {
            "name": "test_host",
            "cluster": {
                "name": "test_cluster",
                "dasEnabled": true
                
            }
        }
    }
    results := concerns with input as mock_vm
    count(results) == 1
}
