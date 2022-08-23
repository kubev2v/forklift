package io.konveyor.forklift.ovirt
 
test_without_ksm_enabled {
    mock_vm := { "name": "test",
                 "cluster": {
                      "ksmEnabled": false
                  }
                }
    results = concerns with input as mock_vm
    count(results) == 0
}

test_with_ksm_enabled {
    mock_vm := { "name": "test",
                 "cluster": {
                      "ksmEnabled": true
                  }
                }
    results = concerns with input as mock_vm
    count(results) == 1
}