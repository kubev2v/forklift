# OS Upgrade Recommendation Policy
# This policy provides upgrade recommendations for unsupported operating systems.

package io.konveyor.forklift.vmware

import rego.v1


# Map guestId patterns to os_upgrade_map source OSs format
guest_id_to_os_name := {
    "rhel6": "Red Hat Enterprise Linux 6",
    "centos7": "CentOs 7",
    "centos8": "CentOs 8",
    "centos9": "CentOs 9",
    "amazonlinux2": "Amazon linux 2",
}

# Map of unsupported operating systems to their recommended upgrade targets
os_upgrade_map := {
    "red hat enterprise linux 6": "Red Hat Enterprise Linux 7",
    "centos 7": "Red Hat Enterprise Linux 7",
    "centos 8": "Red Hat Enterprise Linux 8",
    "centos 9": "Red Hat Enterprise Linux 9",
    "amazon linux 2": "Red Hat Enterprise Linux 8",
}

# Get the OS name to use for matching
get_os_name := os_name if {
    # 1. Prefer guestNameFromVmwareTools if non-empty
    is_string(input.guestNameFromVmwareTools)
    input.guestNameFromVmwareTools != ""
    os_name := input.guestNameFromVmwareTools

} else := os_name if {
    # 2. Fallback to guestName if non-empty
    is_string(input.guestName)
    input.guestName != ""
    os_name := input.guestName

} else := os_name if {
    # 3. Fallback to guestId mapping
    is_string(input.guestId)
    lower_id := lower(input.guestId)
    some id_pattern, name in guest_id_to_os_name
    contains(lower_id, id_pattern)
    os_name := name
}

# Find the upgrade target for the given OS name
find_upgrade_target(os_name) := upgrade_target if {
    lower_os := lower(os_name)
    some source_os, target in os_upgrade_map
    contains(lower_os, source_os)
    upgrade_target := target
}

# Add concern for OS upgrade recommendation
concerns contains flag if {
    os_name := get_os_name
    upgrade_target := find_upgrade_target(os_name)
    flag := {
        "id": "vmware.os.upgrade.recommendation",
        "category": "Information",
        "label": "OS Upgrade Recommendation",
        "assessment": sprintf("%s must be upgraded to a supported version in order to be supported by Red Hat. The minimum recommended version is %s.", [os_name, upgrade_target]),
    }
}

