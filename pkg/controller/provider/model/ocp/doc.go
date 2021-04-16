package ocp

//
// Build all models.
func All() []interface{} {
	return []interface{}{
		&Provider{},
		&NetworkAttachmentDefinition{},
		&StorageClass{},
		&Namespace{},
		&VM{},
	}
}
