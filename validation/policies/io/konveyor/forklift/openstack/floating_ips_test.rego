package io.konveyor.forklift.openstack

import rego.v1

test_without_floating_ips if {
	mock_vm := {
		"name": "test",
		"addresses": {
			"network1": [
				{"OS-EXT-IPS:type": "fixed"},
				{"OS-EXT-IPS:type": "fixed"},
			],
			"network2": [
				{"OS-EXT-IPS:type": "fixed"},
				{"OS-EXT-IPS:type": "fixed"},
			],
		},
	}
	results := concerns with input as mock_vm
	count(results) == 0
}

test_with_floating_ips if {
	mock_vm := {
		"name": "test",
		"addresses": {
			"network1": [
				{"OS-EXT-IPS:type": "fixed"},
				{"OS-EXT-IPS:type": "fixed"},
				{"OS-EXT-IPS:type": "floating"},
			],
			"network2": [
				{"OS-EXT-IPS:type": "fixed"},
				{"OS-EXT-IPS:type": "fixed"},
			],
		},
	}
	results := concerns with input as mock_vm
	count(results) == 1
}
