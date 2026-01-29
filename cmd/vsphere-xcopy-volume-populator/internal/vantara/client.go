package vantara

// VantaraClient defines the interface for interacting with Vantara storage REST API
// This interface abstracts the HTTP client implementation for better testability
type VantaraClient interface {
	// Session management
	Connect() error
	Disconnect() error

	// LUN operations
	GetLdev(ldevId string) (*LdevResponse, error)

	// Path operations
	AddPath(ldevId string, portId string, hostGroupNumber string) error
	DeletePath(ldevId string, portId string, hostGroupNumber string, lunId string) error

	// Port operations
	GetPortDetails() (*PortDetailsResponse, error)

	// Snapshot and clone operations
	CreateCloneLdev(snapshotGroupName string, snapshotPoolId string, pvolLdevId string, svolLdevId string, copySpeed string) error
	GetClonePairs(snapshotGroupName string, pvolLdevId string) (*ClonePairResponse, error)
}

// LdevResponse represents the response from GetLdev API call
type LdevResponse struct {
	LdevId float64       `json:"ldevId"`
	PoolId float64       `json:"poolId"`
	NaaId  string        `json:"naaId"`
	Ports  []PortMapping `json:"ports"`
}

// PortMapping represents a port mapping for a LUN
type PortMapping struct {
	PortId          string  `json:"portId"`
	HostGroupName   string  `json:"hostGroupName"`
	HostGroupNumber float64 `json:"hostGroupNumber"`
	Lun             float64 `json:"lun"`
}

// CloneDataEntry represents a single entry in the ClonePairResponse
type CloneDataEntry struct {
	SnapshotGroupName string  `json:"snapshotGroupName"`
	PvolLdevId        float64 `json:"pvolLdevId"`
	MuNumber          float64 `json:"muNumber"`
	Status            string  `json:"status"`
}

type ClonePairResponse struct {
	Data []CloneDataEntry `json:"data"`
}

// PortDetailsResponse represents the response from GetPortDetails API call
type PortDetailsResponse struct {
	Data []DataEntry `json:"data"`
}
