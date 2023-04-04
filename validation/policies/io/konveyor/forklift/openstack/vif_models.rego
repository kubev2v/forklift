package io.konveyor.forklift.openstack

import future.keywords.if

default invalid_vif_model = false

invalid_vif_model if {
	not regex.match(`e1000|e1000e|rtl8139|virtio|ne2k_pci|pcnet`, input.image.properties.hw_vif_model)
}

concerns[flag] {
	invalid_vif_model
	flag := {
		"category": "Warning",
		"label": "Unsupported VIF model detected",
		"assessment": "The VIF model is not supported by OpenShift Virtualization (only e1000, e1000e, rtl8139, ne2k_pci, pcnet and virtio VIF models are currently supported). The migrated VM will be given a virtio VIF model.",
	}
}
