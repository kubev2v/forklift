package types

// VM represents a Hyper-V virtual machine.
type VM struct {
	UUID          string         `json:"uuid"`
	Name          string         `json:"name"`
	PowerState    string         `json:"powerState"`
	CpuCount      int            `json:"cpuCount"`
	MemoryMB      int64          `json:"memoryMB"`
	Firmware      string         `json:"firmware"`
	GuestOS       string         `json:"guestOS,omitempty"`
	TpmEnabled    bool           `json:"tpmEnabled"`
	SecureBoot    bool           `json:"secureBoot"`
	HasCheckpoint bool           `json:"hasCheckpoint"`
	Disks         []Disk         `json:"disks"`
	NICs          []NIC          `json:"nics"`
	GuestNetworks []GuestNetwork `json:"guestNetworks,omitempty"`
	Concerns      []Concern      `json:"concerns,omitempty"`
}

// Disk represents a Hyper-V virtual disk.
type Disk struct {
	ID          string `json:"id"`
	WindowsPath string `json:"windowsPath"`
	SMBPath     string `json:"smbPath"`
	Capacity    int64  `json:"capacity"`
	Format      string `json:"format"`
	RCTEnabled  bool   `json:"rctEnabled"`
}

// NIC represents a Hyper-V virtual network interface.
type NIC struct {
	Name        string `json:"name"`
	MAC         string `json:"mac"`
	DeviceIndex int    `json:"deviceIndex"`
	NetworkUUID string `json:"networkUUID"`
	NetworkName string `json:"networkName"`
}

// GuestNetwork represents guest OS network configuration collected via KVP Exchange.
type GuestNetwork struct {
	MAC          string   `json:"mac"`
	IP           string   `json:"ip"`
	DeviceIndex  int      `json:"deviceIndex"`
	Origin       string   `json:"origin"`
	PrefixLength int32    `json:"prefix"`
	DNS          []string `json:"dns"`
	Gateway      string   `json:"gateway"`
}

// Network represents a Hyper-V virtual network/switch.
type Network struct {
	UUID       string `json:"uuid"`
	Name       string `json:"name"`
	SwitchType string `json:"switchType"`
}

// Storage represents a Hyper-V storage location.
type Storage struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Path     string `json:"path"`
	Capacity int64  `json:"capacity,omitempty"`
	Free     int64  `json:"free,omitempty"`
}

// Concern represents a migration concern/warning for a VM.
type Concern struct {
	Category string `json:"category"`
	Label    string `json:"label"`
	Message  string `json:"message"`
}
