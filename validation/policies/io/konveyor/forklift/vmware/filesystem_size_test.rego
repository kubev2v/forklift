package io.konveyor.forklift.vmware

# --- Test Cases for Sufficient Space (Passing) ---

test_all_ok {
    mock_vm := {
        "guestDisks": [
            {"diskPath": "/", "freeSpace": 200 * 1024 * 1024},
            {"diskPath": "/boot", "freeSpace": 100 * 1024 * 1024},
            {"diskPath": "C:\\", "freeSpace": 150 * 1024 * 1024},
            {"diskPath": "/data", "freeSpace": 50 * 1024 * 1024}
        ]
    }
    results = concerns with input as mock_vm
    count(results) == 0
}

test_exact_minimum_space {
    mock_vm := {
        "guestDisks": [
            {"diskPath": "/", "freeSpace": 100 * 1024 * 1024},
            {"diskPath": "/boot", "freeSpace": 50 * 1024 * 1024},
            {"diskPath": "C:\\", "freeSpace": 100 * 1024 * 1024},
            {"diskPath": "/data", "freeSpace": 10 * 1024 * 1024}
        ]
    }
    results = concerns with input as mock_vm
    count(results) == 0
}

# --- Test Cases for Insufficient Free Space (Failing) ---

test_insufficient_root_space {
    mock_vm := {
        "guestDisks": [
            {"diskPath": "/", "freeSpace": 99 * 1024 * 1024}
        ]
    }
    results = concerns with input as mock_vm
    count(results) == 1
    results[_].id == "vmware.guestDisks.freespace"
}

test_insufficient_boot_space {
    mock_vm := {
        "guestDisks": [
            {"diskPath": "/boot", "freeSpace": 49 * 1024 * 1024}
        ]
    }
    results = concerns with input as mock_vm
    count(results) == 1
    results[_].id == "vmware.guestDisks.freespace"
}

test_insufficient_windows_c_backslash_space {
    mock_vm := {
        "guestDisks": [
            {"diskPath": "C:\\", "freeSpace": 99 * 1024 * 1024}
        ]
    }
    results = concerns with input as mock_vm
    count(results) == 1
    results[_].id == "vmware.guestDisks.freespace"
}

test_insufficient_windows_c_space {
    mock_vm := {
        "guestDisks": [
            {"diskPath": "C:", "freeSpace": 99 * 1024 * 1024}
        ]
    }
    results = concerns with input as mock_vm
    count(results) == 1
    results[_].id == "vmware.guestDisks.freespace"
}

test_insufficient_windows_c_forward_slash_space {
    mock_vm := {
        "guestDisks": [
            {"diskPath": "C:/", "freeSpace": 99 * 1024 * 1024}
        ]
    }
    results = concerns with input as mock_vm
    count(results) == 1
    results[_].id == "vmware.guestDisks.freespace"
}

test_sufficient_windows_d_space {
    mock_vm := {
        "guestDisks": [
            {"diskPath": "D:\\", "freeSpace": 99 * 1024 * 1024}
        ]
    }
    results = concerns with input as mock_vm
    count(results) == 0
}
test_insufficient_windows_d_space {
    mock_vm := {
        "guestDisks": [
            {"diskPath": "D:\\", "freeSpace": 9 * 1024 * 1024}
        ]
    }
    results = concerns with input as mock_vm
    count(results) == 1
    results[_].id == "vmware.guestDisks.freespace"
}

test_insufficient_other_space {
    mock_vm := {
        "guestDisks": [
            {"diskPath": "/var/log", "freeSpace": 9 * 1024 * 1024}
        ]
    }
    results = concerns with input as mock_vm
    count(results) == 1
    results[_].id == "vmware.guestDisks.freespace"
}

# --- Test Case for Multiple Failures ---

test_multiple_failures {
    mock_vm := {
        "guestDisks": [
            {"diskPath": "/", "freeSpace": 50 * 1024 * 1024},
            {"diskPath": "/boot", "freeSpace": 40 * 1024 * 1024}
        ]
    }
    results = concerns with input as mock_vm
    count(results) == 2
}

# --- Test Cases for Edge Cases ---

test_no_guest_disks {
    mock_vm := {
        "guestDisks": []
    }
    results = concerns with input as mock_vm
    count(results) == 0
}

