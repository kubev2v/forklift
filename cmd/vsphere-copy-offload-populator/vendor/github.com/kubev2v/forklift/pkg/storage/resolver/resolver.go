package resolver

type CsiImportPlugin interface {
	Resolve(backing *DiskBacking) (annotations map[string]string, found bool, err error)
}

type CsiImportPluginFunc func(backing *DiskBacking) (map[string]string, bool, error)

func (f CsiImportPluginFunc) Resolve(backing *DiskBacking) (map[string]string, bool, error) {
	return f(backing)
}
