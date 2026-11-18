package resolver

// CsiImportPlugin is implemented by each storage vendor sub-package.
// Mirrors xcopy's StorageApi/VVolCapable/RDMCapable interface pattern:
// the interface lives in the shared package, vendor sub-packages implement it,
// and the switch that instantiates concrete types lives in the caller (csi_import.go).
type CsiImportPlugin interface {
	// Resolve returns PVC annotations for CSI import, or (nil, nil) if the vendor
	// cannot handle this disk type (caller should fall through to other migration paths).
	Resolve(backing *DiskBacking) (map[string]string, error)
}
