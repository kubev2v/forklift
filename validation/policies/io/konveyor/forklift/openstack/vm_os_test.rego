package io.konveyor.forklift.openstack

import rego.v1

test_without_os_distro_defined if {
	mock_vm := {
		"name": "test",
		"image": {"properties": {"os_version": "6"}},
	}
	results = concerns with input as mock_vm
	count(results) == 0
}

test_without_os_version_defined if {
	mock_vm := {
		"name": "test",
		"image": {"properties": {"os_distro": "rhel"}},
	}
	results = concerns with input as mock_vm
	count(results) == 0
}

test_with_unsupported_os_distro if {
	mock_vm := {
		"name": "test",
		"image": {"properties": {"os_distro": "debian", "os_version": "10"}},
	}
	results = concerns with input as mock_vm
	count(results) == 1
}

test_with_unsupported_os_version if {
	mock_vm := {
		"name": "test",
		"image": {"properties": {"os_distro": "rhel", "os_version": "6"}},
	}
	results = concerns with input as mock_vm
	count(results) == 1
}

test_with_supported_rhel if {
	mock_vm := {
		"name": "test",
		"image": {"properties": {"os_distro": "rhel", "os_version": "9"}},
	}
	results = concerns with input as mock_vm
	count(results) == 0
}

test_with_supported_centos if {
	mock_vm := {
		"name": "test",
		"image": {"properties": {"os_distro": "centos", "os_version": "8-stream"}},
	}
	results = concerns with input as mock_vm
	count(results) == 0
}

test_with_supported_fedora if {
	mock_vm := {
		"name": "test",
		"image": {"properties": {"os_distro": "fedora", "os_version": "38"}},
	}
	results = concerns with input as mock_vm
	count(results) == 0
}

test_with_supported_windows if {
	mock_vm := {
		"name": "test",
		"image": {"properties": {"os_distro": "windows", "os_version": "10"}},
	}
	results = concerns with input as mock_vm
	count(results) == 0
}
