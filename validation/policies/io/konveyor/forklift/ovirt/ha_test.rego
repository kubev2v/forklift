package io.konveyor.forklift.ovirt
 
test_without_ha_enabled {
    mock_vm := { "name": "test",
                 "haEnabled": false
                }
    results = concerns with input as mock_vm
    count(results) == 0
}

test_with_ha_enabled {
    mock_vm := { "name": "test",
                 "haEnabled": true
                }
    results = concerns with input as mock_vm
    count(results) == 1
}