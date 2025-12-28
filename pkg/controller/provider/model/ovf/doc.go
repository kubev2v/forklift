package ovf

import (
	"github.com/kubev2v/forklift/pkg/controller/provider/model/ocp"
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
