package io.konveyor.forklift.ovirt

import rego.v1

disks_with_scsi_reservation contains i if {
	some i
	input.diskAttachments[i].scsiReservation == true
}

concerns contains flag if {
	count(disks_with_scsi_reservation) > 0
	flag := {
		"id": "ovirt.disk.scsi_reservation.enabled",
		"category": "Warning",
		"label": "Shared disk detected",
		"assessment": "The VM has a disk that is shared. Shared disks are not currently supported by OpenShift Virtualization.",
	}
}
