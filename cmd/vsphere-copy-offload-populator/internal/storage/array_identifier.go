package storage

// ArrayIdentifier determines if a storage device belongs to this array.
type ArrayIdentifier interface {
	// TargetPorts returns this array's own SAN target ports (fc.<wwpn> / iqn), called once per array.
	TargetPorts() ([]string, error)
}

// SciniAware indicates PowerFlex/scini.
type SciniAware interface {
	SciniRequired() bool
}
