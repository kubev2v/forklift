package nutanix

// All models.
func All() []interface{} {
	return []interface{}{
		&Cluster{},
		&Host{},
		&Network{},
		&StorageContainer{},
		&VM{},
		&Image{},
	}
}
