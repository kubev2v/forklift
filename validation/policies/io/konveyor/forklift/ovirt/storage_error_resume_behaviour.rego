package io.konveyor.forklift.ovirt

import rego.v1

default storage_error_resume_behaviour := false

storage_error_resume_behaviour if {
	input.storageErrorResumeBehaviour != "auto_resume"
}

concerns contains flag if {
	storage_error_resume_behaviour
	flag := {
		"id": "ovirt.storage.resume_behavior.unsupported",
		"category": "Information",
		"label": "VM storage error resume behavior",
		"assessment": sprintf("The VM has storage error resume behavior set to '%v', which is not currently supported by OpenShift Virtualization", [input.storageErrorResumeBehaviour]),
	}
}
