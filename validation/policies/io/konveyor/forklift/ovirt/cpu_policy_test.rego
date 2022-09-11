package io.konveyor.forklift.ovirt

test_with_none {
    mock_vm := {
        "name": "test",
        "cpuPinningPolicy": "none"
    }
    results := concerns with input as mock_vm
    count(results) == 0
}

test_with_dedicated {
    mock_vm := {
        "name": "test",
        "cpuPinningPolicy": "dedicated"
    }
    results := concerns with input as mock_vm
    count(results) == 0
}

test_with_manual {
    mock_vm := {
        "name": "test",
        "cpuPinningPolicy": "manual",
        "cpuAffinity": [0,2]
    }
    results := concerns with input as mock_vm
    count(results) == 1
}

test_with_resize_and_pin_numa {
    mock_vm := {
        "name": "test",
        "cpuPinningPolicy": "resize_and_pin_numa"
    }
    results := concerns with input as mock_vm
    count(results) == 1
}

test_with_isolate_threads {
    mock_vm := {
        "name": "test",
        "cpuPinningPolicy": "isolate_threads"
    }
    results := concerns with input as mock_vm
    count(results) == 1
}
