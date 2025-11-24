package goscaleio

// VTreeDetails defines struct for VTrees
type VTreeDetails struct {
	CompressionMethod  string             `json:"compressionMethod"`
	DataLayout         string             `json:"dataLayout"`
	ID                 string             `json:"id"`
	InDeletion         bool               `json:"inDeletion"`
	Name               string             `json:"name"`
	RootVolumes        []string           `json:"rootVolumes"`
	StoragePoolID      string             `json:"storagePoolId"`
	Links              []*Link            `json:"links"`
	VtreeMigrationInfo VTreeMigrationInfo `json:"vtreeMigrationInfo"`
}

// VTreeMigrationInfo defines struct for VTree migration
type VTreeMigrationInfo struct {
	DestinationStoragePoolID string `json:"destinationStoragePoolId"`
	MigrationPauseReason     string `json:"migrationPauseReason"`
	MigrationQueuePosition   int64  `json:"migrationQueuePosition"`
	MigrationStatus          string `json:"migrationStatus"`
	SourceStoragePoolID      string `json:"sourceStoragePoolId"`
	ThicknessConversionType  string `json:"thicknessConversionType"`
}

// VTreeQueryBySelectedIDsParam defines struct for specifying Vtree IDs
type VTreeQueryBySelectedIDsParam struct {
	IDs []string `json:"ids"`
}
