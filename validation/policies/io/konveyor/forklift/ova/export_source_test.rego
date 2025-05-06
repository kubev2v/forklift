package io.konveyor.forklift.ova

test_with_unsupported_source {
    mock_vm := { "name": "test", "ovaSource": "Unknown" }
    results = concerns with input as mock_vm
    count(results) == 1
}

test_with_supported_source {
    mock_vm := { "name": "test", "ovaSource": "VMware" }
    results = concerns with input as mock_vm
    count(results) == 0
}
