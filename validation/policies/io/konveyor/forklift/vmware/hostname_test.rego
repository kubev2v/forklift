package io.konveyor.forklift.vmware

import rego.v1

import data.io.konveyor.forklift.vmware.concerns

# Test for empty hostname
test_empty_hostName if {
	mock_vm := {
		"name": "test",
		"hostName": "",
	}
	results = concerns with input as mock_vm
	count(results) == 1
}

test_localhost_hostname if {
	mock_vm := {"hostName": "localhost.localdomain"}
	results = concerns with input as mock_vm
	count(results) == 1
}
