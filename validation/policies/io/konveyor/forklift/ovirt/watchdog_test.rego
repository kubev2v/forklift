package io.konveyor.forklift.ovirt
 
test_without_watchdog {
    mock_vm := { "name": "test",
                 "watchDogs": []
                }
    results = concerns with input as mock_vm
    count(results) == 0
}

test_with_watchdog {
    mock_vm := { "name": "test",
                 "watchDogs": [
                     { "model": "i6300esb", "action": "reset" }
                  ]
                }
    results = concerns with input as mock_vm
    count(results) == 1
}