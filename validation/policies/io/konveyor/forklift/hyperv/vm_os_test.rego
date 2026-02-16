package io.konveyor.forklift.hyperv

import rego.v1

test_supported_windows_server_2019 if {
	mock_vm := {"name": "test-vm", "guestOS": "Windows Server 2019", "guestNetworks": [{"ip": "10.0.0.1"}], "disks": [{"Name": "disk-0", "Capacity": 1000}]}
	results := concerns with input as mock_vm
	not any_os_concern(results)
}

any_os_concern(results) if {
	some result in results
	result.id == "hyperv.os.unsupported"
}

test_supported_rhel8 if {
	mock_vm := {"name": "test-vm", "guestOS": "Red Hat Enterprise Linux 8", "guestNetworks": [{"ip": "10.0.0.1"}], "disks": [{"Name": "disk-0", "Capacity": 1000}]}
	results := concerns with input as mock_vm
	not any_os_concern(results)
}

test_unsupported_os if {
	mock_vm := {"name": "test-vm", "guestOS": "Ubuntu 20.04", "guestNetworks": [{"ip": "10.0.0.1"}], "disks": [{"Name": "disk-0", "Capacity": 1000}]}
	results := concerns with input as mock_vm
	some result in results
	result.id == "hyperv.os.unsupported"
}

test_no_guest_os if {
	mock_vm := {"name": "test-vm", "guestOS": "", "guestNetworks": [{"ip": "10.0.0.1"}], "disks": [{"Name": "disk-0", "Capacity": 1000}]}
	results := concerns with input as mock_vm
	not any_os_concern(results)
}
