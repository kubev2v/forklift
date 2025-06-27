package io.konveyor.forklift.vmware

change_tracking_disabled {
    input.changeTrackingEnabled == false
}

concerns[flag] {
    change_tracking_disabled
    flag := {
        "id": "vmware.changed_block_tracking.disabled",
        "category": "Warning",
        "label": "Changed Block Tracking (CBT) not enabled",
        "assessment": "For VM warm migration, Changed Block Tracking (CBT) must be enabled in VMware."
    }
}
