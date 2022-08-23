package io.konveyor.forklift.ovirt

default storage_error_resume_behaviour = false

storage_error_resume_behaviour = true {
    input.storageErrorResumeBehaviour != "auto_resume"
}

concerns[flag] {
    storage_error_resume_behaviour
    flag := {
        "category": "Information",
        "label": "VM storage error resume behavior",
        "assessment": sprintf("The VM has storage error resume behavior set to '%v', which is not currently supported by OpenShift Virtualization", [input.storageErrorResumeBehaviour])
    }
}
