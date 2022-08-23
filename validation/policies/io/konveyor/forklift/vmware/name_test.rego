package io.konveyor.forklift.vmware

test_valid_vm_name {
    mock_vm := { "name": "test" }
    results := concerns with input as mock_vm
    count(results) == 0
}

test_vm_name_too_long {
    mock_vm := { "name": "my-vm-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx" }
    results := concerns with input as mock_vm
    count(results) == 1
}

test_vm_name_invalid_char_underscore {
    mock_vm := { "name": "my_vm" }
    results := concerns with input as mock_vm
    count(results) == 1
}

test_vm_name_invalid_char_slash {
    mock_vm := { "name": "my/vm" }
    results := concerns with input as mock_vm
    count(results) == 1
}
