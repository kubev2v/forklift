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
}

// LdevResponse represents the response from GetLdev API call
type LdevResponse struct {
	LdevId float64       `json:"ldevId"`
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

// PortDetailsResponse represents the response from GetPortDetails API call
type PortDetailsResponse struct {
	Data []DataEntry `json:"data"`
}
