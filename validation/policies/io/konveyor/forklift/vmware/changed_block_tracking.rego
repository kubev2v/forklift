package io.konveyor.forklift.vmware

change_tracking_disabled {
    input.changeTrackingEnabled == false
}

concerns[flag] {
    change_tracking_disabled
    flag := {
        "category": "Warning",
        "label": "Changed Block Tracking (CBT) not enabled",
        "assessment": "Changed Block Tracking (CBT) has not been enabled on this VM. This feature is a prerequisite for VM warm migration."
    }
}
