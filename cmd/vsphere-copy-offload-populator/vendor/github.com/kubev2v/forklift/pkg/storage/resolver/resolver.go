package resolver

type CsiImportPlugin interface {
	Resolve(backing *DiskBacking) (annotations map[string]string, found bool, err error)
}
