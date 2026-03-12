package io.konveyor.forklift.hyperv

import rego.v1

test_disk_with_smb_path if {
	mock_vm := {
		"name": "test-vm",
		"disks": [{
			"name": "disk-0",
			"capacity": 1000,
			"windowsPath": "C:\\VMs\\test.vhdx",
			"smbPath": "/hyperv/test.vhdx",
		}],
	}
	results := concerns with input as mock_vm
	not any_smb_path_concern(results)
}

test_disk_missing_smb_path if {
	mock_vm := {
		"name": "test-vm",
		"disks": [{
			"name": "disk-0",
			"capacity": 1000,
			"windowsPath": "C:\\VMs\\test.vhdx",
			"smbPath": "",
		}],
	}
	results := concerns with input as mock_vm
	some result in results
	result.id == "hyperv.disk.smb_path.missing"
}

test_disk_null_smb_path if {
	mock_vm := {
		"name": "test-vm",
		"disks": [{
			"name": "disk-0",
			"capacity": 1000,
			"windowsPath": "C:\\VMs\\test.vhdx",
		}],
	}
	results := concerns with input as mock_vm
	some result in results
	result.id == "hyperv.disk.smb_path.missing"
}

test_multiple_disks_one_missing if {
	mock_vm := {
		"name": "test-vm",
		"disks": [
			{
				"name": "disk-0",
				"capacity": 1000,
				"windowsPath": "C:\\VMs\\disk0.vhdx",
				"smbPath": "/hyperv/disk0.vhdx",
			},
			{
				"name": "disk-1",
				"capacity": 2000,
				"windowsPath": "D:\\Other\\disk1.vhdx",
				"smbPath": "",
			},
		],
	}
	results := concerns with input as mock_vm
	count([r | some r in results; r.id == "hyperv.disk.smb_path.missing"]) == 1
}

any_smb_path_concern(results) if {
	some result in results
	result.id == "hyperv.disk.smb_path.missing"
}
