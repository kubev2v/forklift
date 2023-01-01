package openstack

import (
	"github.com/konveyor/forklift-controller/pkg/controller/provider/model/ocp"
)

// Build all models.
func All() []interface{} {
	return []interface{}{
		&ocp.Provider{},
		&Region{},
		&Project{},
		&Flavor{},
		&Image{},
		&VM{},
		&Volume{},
		&Snapshot{},
	}
}
