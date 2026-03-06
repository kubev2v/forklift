package driver

import "context"

// DomainState represents VM power state
type DomainState int

const (
	DOMAIN_NOSTATE     DomainState = 0
	DOMAIN_RUNNING     DomainState = 1
	DOMAIN_BLOCKED     DomainState = 2
	DOMAIN_PAUSED      DomainState = 3
	DOMAIN_SHUTDOWN    DomainState = 4
	DOMAIN_SHUTOFF     DomainState = 5
	DOMAIN_CRASHED     DomainState = 6
	DOMAIN_PMSUSPENDED DomainState = 7
)

// DomainInfo contains VM resource information
type DomainInfo struct {
	State     DomainState
	MaxMem    uint64 // Maximum memory in KB
	Memory    uint64 // Current memory in KB
	NrVirtCpu uint16 // Number of virtual CPUs
}

// HyperVDriver abstracts Hyper-V host operations
type HyperVDriver interface {
	// Connection management
	Connect() error
	Close() error
	IsAlive() (bool, error)

	ListAllDomains() ([]Domain, error)
	LookupDomainByName(name string) (Domain, error)
	LookupDomainByUUIDString(uuid string) (Domain, error)

	ListAllNetworks() ([]Network, error)
	LookupNetworkByUUIDString(uuid string) (Network, error)

	// Raw command execution
	ExecuteCommand(command string) (string, error)
}

// Domain represents a virtual machine
type Domain interface {
	GetName() (string, error)
	GetUUIDString() (string, error)
	GetState() (state DomainState, reason int, err error)
	GetInfo() (*DomainInfo, error)
	GetGeneration() (int, error)
	GetDisks() ([]DiskInfo, error)
	GetNICs() ([]NICInfo, error)
	Shutdown(ctx context.Context) error
	Free() error
}

// Network represents a Hyper-V virtual switch/network
type Network interface {
	GetName() (string, error)
	GetUUIDString() (string, error)
	GetSwitchType() (string, error)
	Free() error
}

type DiskInfo struct {
	Path           string // Windows path
	ControllerType string // SCSI, IDE
	ControllerNum  int
	ControllerLoc  int
}

type NICInfo struct {
	Name       string
	MACAddress string
	SwitchName string
}
