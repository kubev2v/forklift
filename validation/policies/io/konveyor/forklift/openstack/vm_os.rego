package io.konveyor.forklift.openstack

import future.keywords.if
import future.keywords.in

default is_supported_redhat_guest = false

is_supported_redhat_guest if {
	regex.match(`rhel|centos`, input.image.properties.os_distro)
	regex.match(`^9|^8|^7`, input.image.properties.os_version)
}

default is_supported_windows_guest = false

is_supported_windows_guest if {
	regex.match(`windows`, input.image.properties.os_distro)
	regex.match(`2008|2012|2016|2019|2022|2k8|2k12|2k16|2k19|2k22|^7|^8|^10|^11`, input.image.properties.os_version)
}

default is_supported_fedora_guest = false

is_supported_fedora_guest if {
	regex.match(`fedora`, input.image.properties.os_distro)
	regex.match(`^3[678]$`, input.image.properties.os_version)
}

default has_unsupported_guest_os = false

has_unsupported_guest_os if {
	"os_distro" in object.keys(input.image.properties)
	"os_version" in object.keys(input.image.properties)
	not is_supported_redhat_guest
	not is_supported_windows_guest
	not is_supported_fedora_guest
}

concerns[flag] {
	has_unsupported_guest_os
	flag := {
	    "id": "openstack.os.unsupported",
		"category": "Warning",
		"label": "Unsupported operative system detected",
		"assessment": "The VM is running an operative system that is not currently supported by OpenShift Virtualization.",
	}
}
