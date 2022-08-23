package io.konveyor.forklift.ovirt
 
test_without_boot_menu_enabled {
    mock_vm := {  "name": "test",
                  "bootMenuEnabled" : false
               }
    results = concerns with input as mock_vm
    count(results) == 0
}

test_with_boot_menu_enabled {
    mock_vm := {  "name": "test",
                  "bootMenuEnabled" : true
               }
    results = concerns with input as mock_vm
    count(results) == 1
}