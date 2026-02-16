package hyperv

import (
	"strings"

	hvutil "github.com/kubev2v/forklift/pkg/controller/hyperv"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/hyperv"
)

// VM.
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

// Apply to (update) the model.
func (r *VM) ApplyTo(m *model.VM) {
	m.ID = r.UUID
	m.Name = r.Name
	m.UUID = r.UUID
	m.PowerState = r.PowerState
	m.CpuCount = int32(r.CpuCount)
	m.MemoryMB = int32(r.MemoryMB)
	m.Firmware = r.Firmware
	m.GuestOS = r.GuestOS
	m.TpmEnabled = r.TpmEnabled
	m.SecureBoot = r.SecureBoot
	m.HasCheckpoint = r.HasCheckpoint
	r.addDisks(m)
	r.addNICs(m)
	r.addGuestNetworks(m)
	r.addConcerns(m)
	SortNICsByGuestNetworkOrder(m)
}

func (r *VM) addDisks(m *model.VM) {
	m.Disks = nil
	for _, d := range r.Disks {
		diskName := d.ID
		if d.WindowsPath != "" {
			parts := strings.Split(d.WindowsPath, "\\")
			if len(parts) > 0 {
				diskName = parts[len(parts)-1]
			}
		}
		m.Disks = append(m.Disks, model.Disk{
			Base: model.Base{
				ID:   d.ID,
				Name: diskName,
			},
			WindowsPath: d.WindowsPath,
			SMBPath:     d.SMBPath,
			Datastore: model.Ref{
				Kind: "Storage",
				ID:   hvutil.StorageIDDefault,
			},
			Capacity:   d.Capacity,
			Format:     d.Format,
			RCTEnabled: d.RCTEnabled,
		})
	}
}

func (r *VM) addNICs(m *model.VM) {
	m.NICs = nil
	networkSet := make(map[string]bool)
	for _, n := range r.NICs {
		m.NICs = append(m.NICs, model.NIC{
			Name:        n.Name,
			MAC:         n.MAC,
			DeviceIndex: n.DeviceIndex,
			Network: model.Ref{
				Kind: "Network",
				ID:   n.NetworkUUID,
			},
			NetworkName: n.NetworkName,
		})
		if n.NetworkUUID != "" {
			networkSet[n.NetworkUUID] = true
		}
	}
	m.Networks = nil
	for uuid := range networkSet {
		m.Networks = append(m.Networks, model.Ref{
			Kind: "Network",
			ID:   uuid,
		})
	}
}

func (r *VM) addGuestNetworks(m *model.VM) {
	m.GuestNetworks = nil
	for _, gn := range r.GuestNetworks {
		m.GuestNetworks = append(m.GuestNetworks, model.GuestNetwork{
			MAC:          gn.MAC,
			IP:           gn.IP,
			DeviceIndex:  gn.DeviceIndex,
			Origin:       gn.Origin,
			PrefixLength: gn.PrefixLength,
			DNS:          gn.DNS,
			Gateway:      gn.Gateway,
		})
	}
}

func (r *VM) addConcerns(m *model.VM) {
	m.Concerns = nil
	for _, c := range r.Concerns {
		m.Concerns = append(m.Concerns, model.Concern{
			Category:   c.Category,
			Label:      c.Label,
			Assessment: c.Message,
		})
	}
}

// GuestNetwork.
type GuestNetwork struct {
	MAC          string   `json:"mac"`
	IP           string   `json:"ip"`
	DeviceIndex  int      `json:"deviceIndex"`
	Origin       string   `json:"origin"`
	PrefixLength int32    `json:"prefix"`
	DNS          []string `json:"dns"`
	Gateway      string   `json:"gateway"`
}

// NIC.
type NIC struct {
	Name        string `json:"name"`
	MAC         string `json:"mac"`
	DeviceIndex int    `json:"deviceIndex"`
	NetworkUUID string `json:"networkUUID"`
	NetworkName string `json:"networkName"`
}

// Disk.
type Disk struct {
	ID          string `json:"id"`
	WindowsPath string `json:"windowsPath"`
	SMBPath     string `json:"smbPath"`
	Capacity    int64  `json:"capacity"`
	Format      string `json:"format"`
	RCTEnabled  bool   `json:"rctEnabled"`
}

// Apply to (update) the model.
func (r *Disk) ApplyTo(m *model.Disk) {
	diskName := r.ID
	if r.WindowsPath != "" {
		parts := strings.Split(r.WindowsPath, "\\")
		if len(parts) > 0 {
			diskName = parts[len(parts)-1]
		}
	}
	m.ID = r.ID
	m.Name = diskName
	m.WindowsPath = r.WindowsPath
	m.SMBPath = r.SMBPath
	m.Capacity = r.Capacity
	m.Format = r.Format
	m.RCTEnabled = r.RCTEnabled
	m.Datastore = model.Ref{
		Kind: "Storage",
		ID:   hvutil.StorageIDDefault,
	}
}

// Network.
type Network struct {
	UUID       string `json:"uuid"`
	Name       string `json:"name"`
	SwitchType string `json:"switchType"`
}

// Apply to (update) the model.
func (r *Network) ApplyTo(m *model.Network) {
	m.ID = r.UUID
	m.Name = r.Name
	m.UUID = r.UUID
	m.SwitchName = r.Name
	m.SwitchType = r.SwitchType
}

// Storage.
type Storage struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Path     string `json:"path"`
	Capacity int64  `json:"capacity,omitempty"`
	Free     int64  `json:"free,omitempty"`
}

// Apply to (update) the model.
func (r *Storage) ApplyTo(m *model.Storage) {
	m.ID = r.ID
	m.Name = r.Name
	m.Type = r.Type
	m.Path = r.Path
	m.Capacity = r.Capacity
	m.Free = r.Free
}

// Concern.
type Concern struct {
	Category string `json:"category"`
	Label    string `json:"label"`
	Message  string `json:"message"`
}
