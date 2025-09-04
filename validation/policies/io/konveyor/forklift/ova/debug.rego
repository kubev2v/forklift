package io.konveyor.forklift.ova

import rego.v1

debug if {
	trace(sprintf("** debug ** vm name: %v", [input.name]))
}
