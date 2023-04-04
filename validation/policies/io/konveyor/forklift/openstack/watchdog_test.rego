package io.konveyor.forklift.openstack

test_without_watchdog {
	mock_vm := {
		"name": "test",
		"flavor": {"extraSpecs": {}},
		"image": {"properties": {}},
	}
	results = concerns with input as mock_vm
	count(results) == 0
}

test_with_flavor_watchdog {
	mock_vm := {
		"name": "test",
		"flavor": {"extraSpecs": {"hw:watchdog_action": "reset"}},
	}
	results = concerns with input as mock_vm
	count(results) == 1
}

test_with_image_watchdog {
	mock_vm := {
		"name": "test",
		"image": {"properties": {"hw_watchdog_action": "reset"}},
	}
	results = concerns with input as mock_vm
	count(results) == 1
}

test_with_flavor_and_image_watchdogs {
	mock_vm := {
		"name": "test",
		"flavor": {"extraSpecs": {"hw:watchdog_action": "reset"}},
		"image": {"properties": {"hw_watchdog_action": "reset"}},
	}
	results = concerns with input as mock_vm
	count(results) == 1
}
