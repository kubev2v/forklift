package hyperv

import (
	"github.com/kubev2v/forklift/pkg/controller/provider/model/ocp"
)

// Build all models.
func All() []interface{} {
	return []interface{}{
		&ocp.Provider{},
		&Cluster{},
		&Host{},
		&Network{},
		&Storage{},
		&VM{},
		&Disk{},
	}
}
