package io.konveyor.forklift.vmware

test_with_changed_block_tracking_enabled {
    mock_vm := { "name": "test", "changeTrackingEnabled": true }
    results := concerns with input as mock_vm
    count(results) == 0
}

test_with_changed_block_tracking_disabled {
    mock_vm := { "name": "test", "changeTrackingEnabled": false }
    results := concerns with input as mock_vm
    count(results) == 1
}
