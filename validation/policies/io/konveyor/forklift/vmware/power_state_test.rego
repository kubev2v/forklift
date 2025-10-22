package io.konveyor.forklift.vmware

test_with_power_state_powered_on {
    mock_vm := { "name": "test", "powerState": "poweredOn" }
    results := concerns with input as mock_vm
    count(results) == 0
}

test_with_power_state_powered_off {
    mock_vm := { "name": "test", "powerState": "poweredOff" }
    results := concerns with input as mock_vm
    count(results) == 1
}