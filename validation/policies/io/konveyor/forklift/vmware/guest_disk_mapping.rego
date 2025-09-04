package io.konveyor.forklift.vmware

import rego.v1

# Match any windows disk with missing mappings
invalid_guest_disk_mappings contains idx if {
	some idx

	lower_id := lower(input.guestId)
	is_windows := contains(lower_id, "windows")
	key := object.get(input.guestDisks[idx], "key", 0)
	missing_disk_mapping := key == 0

	is_windows
	missing_disk_mapping
}

# Raise a concern for each invalid disk
concerns contains flag if {
	invalid_guest_disk_mappings[idx]
	disk := input.guestDisks[idx]
	flag := {
		"id": "vmware.guestDisks.key.not_found",
		"category": "Information",
		"label": sprintf("Missing disk key mapping for '%v'", [disk.diskPath]),
		"assessment": "winDriveLetter cannot be resolved in PVC name templates without a disk key mapping.",
	}
}
