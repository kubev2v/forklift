package io.konveyor.forklift.vmware

import rego.v1

test_with_hotplug_disabled if {
	mock_vm := {
		"name": "test",
		"cpuHotAddEnabled": false,
		"cpuHotRemoveEnabled": false,
		"memoryHotAddEnabled": false,
	}
	results := concerns with input as mock_vm
	count(results) == 0
}

test_with_cpu_hot_add_enabled if {
	mock_vm := {
		"name": "test",
		"cpuHotAddEnabled": true,
		"cpuHotRemoveEnabled": false,
		"memoryHotAddEnabled": false,
	}
	results := concerns with input as mock_vm
	count(results) == 1
}

test_with_cpu_hot_remove_enabled if {
	mock_vm := {
		"name": "test",
		"cpuHotAddEnabled": false,
		"cpuHotRemoveEnabled": true,
		"memoryHotAddEnabled": false,
	}
	results := concerns with input as mock_vm
	count(results) == 1
}

test_with_memory_hot_add_enabled if {
	mock_vm := {
		"name": "test",
		"cpuHotAddEnabled": false,
		"cpuHotRemoveEnabled": false,
		"memoryHotAddEnabled": true,
	}
	results := concerns with input as mock_vm
	count(results) == 1
}
