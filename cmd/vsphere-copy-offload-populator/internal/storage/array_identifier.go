package storage

// ArrayIdentifier determines if a storage device belongs to this array.
type ArrayIdentifier interface {
	// MatchesDevice returns true if the given device name (NAA/EUI canonical
	// name from vSphere, e.g. "naa.600a0980..." or "eui.b4f2d53...") belongs
	// to this storage array instance.
	MatchesDevice(deviceName string) (bool, error)
}
