package io.konveyor.forklift.vmware

import rego.v1

test_with_guest_network_present if {
	mock_vm := {"name": "test", "guestNetworks": [{"ip": "10.2.122.57","mac": "00:50:56:94:0d:b8"}]}
	results := concerns with input as mock_vm
	count(results) == 0
}

test_with_empty_guest_network if {
	mock_vm := {"name": "test", "guestNetworks": []}
	results := concerns with input as mock_vm
	count(results) == 1
}
