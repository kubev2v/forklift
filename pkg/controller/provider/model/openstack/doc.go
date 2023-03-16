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
		&Image{},
		&Flavor{},
		&VM{},
		&Snapshot{},
		&Volume{},
		&VolumeType{},
		&Network{},
		&Subnet{},
	}
}
