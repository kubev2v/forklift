package ova

import (
	"github.com/konveyor/forklift-controller/pkg/controller/provider/model/ocp"
)

// Build all models.
func All() []interface{} {
	return []interface{}{
		&ocp.Provider{},
		&VM{},
		&Network{},
		&Disk{},
		&Storage{},
	}
}
