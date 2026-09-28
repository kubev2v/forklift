package io.konveyor.forklift.hyperv

import rego.v1

test_nic_with_resolved_network if {
	mock_vm := {
		"name": "test-vm",
		"nics": [{
			"name": "nic-0",
			"mac": "00:15:5D:01:DB:01",
			"network": {"id": "a5c4bd9a-5ca9-4bf8-9452-1b97de56d686", "kind": "Network"},
			"networkName": "Lab-External",
		}],
		"disks": [{"name": "disk-0", "capacity": 1000}],
	}
	results := concerns with input as mock_vm
	not any_unresolved_nic_concern(results)
}

test_nic_with_unresolved_network if {
	mock_vm := {
		"name": "test-vm",
		"nics": [{
			"name": "nic-0",
			"mac": "00:15:5D:01:DB:01",
			"network": {"id": "", "kind": "Network"},
			"networkName": "LabSwitch",
		}],
		"disks": [{"name": "disk-0", "capacity": 1000}],
	}
	results := concerns with input as mock_vm
	some result in results
	result.id == "hyperv.nic.unresolved_network"
}

test_nic_disconnected_no_concern if {
	mock_vm := {
		"name": "test-vm",
		"nics": [{
			"name": "nic-0",
			"mac": "00:15:5D:01:DB:01",
			"network": {"id": "", "kind": "Network"},
			"networkName": "",
		}],
		"disks": [{"name": "disk-0", "capacity": 1000}],
	}
	results := concerns with input as mock_vm
	not any_unresolved_nic_concern(results)
}

test_nic_null_network_name_no_concern if {
	mock_vm := {
		"name": "test-vm",
		"nics": [{"name": "nic-0", "mac": "00:15:5D:01:DB:01", "network": {"id": "", "kind": "Network"}}],
		"disks": [{"name": "disk-0", "capacity": 1000}],
	}
	results := concerns with input as mock_vm
	not any_unresolved_nic_concern(results)
}

test_multiple_nics_one_unresolved if {
	mock_vm := {
		"name": "test-vm",
		"nics": [
			{
				"name": "nic-0",
				"mac": "00:15:5D:01:DB:01",
				"network": {"id": "", "kind": "Network"},
				"networkName": "LabSwitch",
			},
			{
				"name": "nic-1",
				"mac": "00:15:5D:01:DB:02",
				"network": {"id": "a5c4bd9a-5ca9-4bf8-9452-1b97de56d686", "kind": "Network"},
				"networkName": "Lab-External",
			},
		],
		"disks": [{"name": "disk-0", "capacity": 1000}],
	}
	results := concerns with input as mock_vm
	count([r | some r in results; r.id == "hyperv.nic.unresolved_network"]) == 1
}

any_unresolved_nic_concern(results) if {
	some result in results
	result.id == "hyperv.nic.unresolved_network"
}
