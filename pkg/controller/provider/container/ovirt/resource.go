package ovirt

import (
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/ovirt"
	"strconv"
)

//
// System.
type System struct {
	Product struct {
		Name    string `json:"name"`
		Vendor  string `json:"vendor"`
		Version struct {
			Build    string `json:"build"`
			Major    string `json:"major"`
			Minor    string `json:"minor"`
			Revision string `json:"revision"`
		} `json:"version"`
	} `json:"product_info"`
}

//
// Ref.
type Ref struct {
	Ref string `json:"href"`
	ID  string `json:"id"`
}

type Base struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

//
// DataCenter.
type DataCenter struct {
	Base
}

//
// Apply to (update) the model.
func (r *DataCenter) ApplyTo(m *model.DataCenter) {
	m.Name = r.Name
	m.Description = r.Description
}

//
// DataCenter (list).
type DataCenterList struct {
	Items []DataCenter `json:"data_center"`
}

//
// Cluster.
type Cluster struct {
	Base
	DataCenter    Ref    `json:"data_center"`
	HaReservation string `json:"ha_reservation"`
}

//
// Apply to (update) the model.
func (r *Cluster) ApplyTo(m *model.Cluster) {
	m.Name = r.Name
	m.Name = r.Description
	m.DataCenter = r.DataCenter.ID
	m.HaReservation, _ = strconv.ParseBool(r.HaReservation)
}

//
// Cluster (list).
type ClusterList struct {
	Items []Cluster `json:"cluster"`
}

//
// Host.
type Host struct {
	Base
	Cluster Ref    `json:"cluster"`
	Status  string `json:"status"`
	OS      struct {
		Type    string `json:"type"`
		Version struct {
			Full string `json:"full_version"`
		} `json:"version"`
	} `json:"os"`
	CPU struct {
		Topology struct {
			Sockets string `json:"sockets"`
			Cores   string `json:"cores"`
		} `json:"topology"`
	} `json:"cpu"`
	KSM struct {
		Enabled string `json:"enabled"`
	} `json:"ksm"`
	SSH struct {
		Thumbprint string `json:"thumbprint"`
	} `json:"ssh"`
	Networks struct {
		Attachment []struct {
			Network Ref `json:"network"`
		} `json:"network_attachment"`
	} `json:"network_attachments"`
}

//
// Apply to (update) the model.
func (r *Host) ApplyTo(m *model.Host) {
	m.Name = r.Name
	m.Description = r.Description
	m.Cluster = r.Cluster.ID
	m.ProductName = r.OS.Type
	m.ProductVersion = r.OS.Version.Full
	m.InMaintenance = r.Status == "maintenance"
	m.KsmEnabled, _ = strconv.ParseBool(r.KSM.Enabled)
	m.Thumbprint = r.SSH.Thumbprint
	m.CpuSockets = r.cpuSockets()
	m.CpuCores = r.cpuCores()
	//
	m.Networks = []string{}
	for _, na := range r.Networks.Attachment {
		m.Networks = append(m.Networks, na.Network.ID)
	}
}

func (r *Host) cpuSockets() int16 {
	n, _ := strconv.ParseInt(r.CPU.Topology.Sockets, 10, 16)
	return int16(n)
}

func (r *Host) cpuCores() int16 {
	n, _ := strconv.ParseInt(r.CPU.Topology.Cores, 10, 16)
	return int16(n)
}

//
// Host (list).
type HostList struct {
	Items []Host `json:"host"`
}

//
// VM.
type VM struct {
	Base
	Cluster Ref `json:"cluster"`
	Host    Ref `json:"host"`
	Guest   struct {
		Distribution string `json:"distribution"`
		Version      struct {
			Full string `json:"full_version"`
		} `json:"version"`
	} `json:"guest_operating_system"`
	CPU struct {
		Tune struct {
			Pin struct {
				List []struct {
					Set string `json:"cpu_set"`
					Cpu string `json:"vcpu"`
				} `json:"vcpu_pin"`
			} `json:"vcpu_pins"`
		} `json:"cpu_tune"`
		Topology struct {
			Sockets string `json:"sockets"`
			Cores   string `json:"cores"`
		} `json:"topology"`
	} `json:"cpu"`
	Memory string `json:"memory"`
	BIOS   struct {
		Type string `json:"type"`
	} `json:"bios"`
	Display struct {
		Type string `json:"type"`
	} `json:"display"`
	NICs struct {
		List []struct {
			ID        string `json:"id"`
			Name      string `json:"name"`
			Interface string `json:"interface"`
			Profile   Ref    `json:"vnic_profile"`
		} `json:"nic"`
	} `json:"nics"`
	Disks struct {
		Attachment []struct {
			ID string `json:"id"`
			Name string 
			Interface string `json:"interface"`
			Disk      Ref    `json:"disk"`
		} `json:"disk_attachment"`
	} `json:"disk_attachments"`
}

//
// Apply to (update) the model.
func (r *VM) ApplyTo(m *model.VM) {
	m.Name = r.Name
	m.Description = r.Description
	m.Cluster = r.Cluster.ID
	m.Host = r.Host.ID
	m.GuestName = r.Guest.Distribution + " " + r.Guest.Version.Full
	m.CpuSockets = r.cpuSockets()
	m.CpuCores = r.cpuCores()
	m.Memory, _ = strconv.ParseInt(r.Memory, 10, 64)
	m.BIOS = r.BIOS.Type
	m.Display = r.Display.Type
	m.CpuAffinity = r.cpuAffinity()
	//
	m.NICs = []model.NIC{}
	for _, n := range r.NICs.List {
		m.NICs = append(
			m.NICs, model.NIC{
				ID:        n.ID,
				Name:      n.Name,
				Interface: n.Interface,
				Profile:   n.Profile.ID,
			})
	}
	//
	m.DiskAttachments = []model.DiskAttachment{}
	for _, da := range r.Disks.Attachment {
		m.DiskAttachments = append(
			m.DiskAttachments,
			model.DiskAttachment{
				ID: da.ID,
				Interface: da.Interface,
				Disk:      da.Disk.ID,
			})
	}
}

func (r *VM) cpuSockets() int16 {
	n, _ := strconv.ParseInt(r.CPU.Topology.Sockets, 10, 16)
	return int16(n)
}

func (r *VM) cpuCores() int16 {
	n, _ := strconv.ParseInt(r.CPU.Topology.Cores, 10, 16)
	return int16(n)
}

func (r *VM) cpuAffinity() (affinity []model.CpuPinning) {
	for _, p := range r.CPU.Tune.Pin.List {
		set, _ := strconv.ParseInt(p.Set, 10, 32)
		cpu, _ := strconv.ParseInt(p.Cpu, 10, 32)
		affinity = append(
			affinity, model.CpuPinning{
				Set: int32(set),
				Cpu: int32(cpu),
			})
	}

	return
}

//
// VM (list).
type VMList struct {
	Items []VM `json:"vm"`
}

//
// Network.
type Network struct {
	Base
	DataCenter Ref `json:"data_center"`
	VLan       Ref `json:"vlan"`
	Usages     struct {
		Usage []string `json:"usage"`
	} `json:"usages"`
	//
	Profiles struct {
		List []struct {
			ID string `json:"id"`
		} `json:"vnic_profile"`
	} `json:"vnic_profiles"`
}

//
// Apply to (update) the model.
func (r *Network) ApplyTo(m *model.Network) {
	m.Name = r.Name
	m.Description = r.Description
	m.DataCenter = r.DataCenter.ID
	m.VLan = r.VLan.ID
	m.Usages = r.Usages.Usage
	//
	m.Profiles = []string{}
	for _, p := range r.Profiles.List {
		m.Profiles = append(m.Profiles, p.ID)
	}
}

//
// Network (list).
type NetworkList struct {
	Items []Network `json:"network"`
}

//
// StorageDomain.
type StorageDomain struct {
	Base
	Type    string `json:"type"`
	Storage struct {
		Type string `json:"type"`
	} `json:"storage"`
	Available  string `json:"available"`
	Used       string `json:"used"`
	DataCenter struct {
		List []Ref `json:"data_center"`
	} `json:"data_centers"`
}

//
// Apply to (update) the model.
func (r *StorageDomain) ApplyTo(m *model.StorageDomain) {
	m.Name = r.Name
	m.Description = r.Description
	m.Type = r.Type
	m.Storage.Type = r.Storage.Type
	m.Available, _ = strconv.ParseInt(r.Available, 10, 64)
	m.Used, _ = strconv.ParseInt(r.Used, 10, 64)
	for _, ref := range r.DataCenter.List {
		m.DataCenter = ref.ID
		break
	}
}

//
// StorageDomain (list).
type StorageDomainList struct {
	Items []StorageDomain `json:"storage_domain"`
}

//
// vNIC profile.
type NICProfile struct {
	Base
	Network Ref `json:"network"`
	QoS     Ref `json:"qos"`
}

//
// Apply to (update) the model.
func (r *NICProfile) ApplyTo(m *model.NICProfile) {
	m.Name = r.Name
	m.Description = r.Description
	m.Network = r.Network.ID
	m.QoS = r.QoS.ID
}

//
// NICProfile (list).
type NICProfileList struct {
	Items []NICProfile `json:"vnic_profile"`
}

//
// Disk.
type Disk struct {
	Base
	Sharable       string `json:"sharable"`
	StorageDomains struct {
		List []Ref `json:"storage_domain"`
	} `json:"storage_domains"`
}

//
// Apply to (update) the model.
func (r *Disk) ApplyTo(m *model.Disk) {
	m.Name = r.Name
	m.Description = r.Description
	m.Shared, _ = strconv.ParseBool(r.Sharable)
	for _, ref := range r.StorageDomains.List {
		m.StorageDomain = ref.ID
		break
	}
}

//
// Disk (list).
type DiskList struct {
	Items []Disk `json:"disk"`
}

//
// Event.
type Event struct {
	ID          string `json:"id"`
	Code        string `json:"code"`
	Description string `json:"description"`
	DataCenter  struct {
		Ref string `json:"href"`
		ID  string `json:"id"`
	} `json:"data_center"`
	Cluster struct {
		Ref string `json:"href"`
		ID  string `json:"id"`
	} `json:"cluster"`
	Host struct {
		Ref string `json:"href"`
		ID  string `json:"id"`
	} `json:"host"`
	VM struct {
		Ref string `json:"href"`
		ID  string `json:"id"`
	} `json:"vm"`
}

func (r *Event) id() (n int) {
	n, _ = strconv.Atoi(r.ID)
	return
}

func (r *Event) code() (n int) {
	n, _ = strconv.Atoi(r.Code)
	return
}

//
// EVent (list).
type EventList struct {
	Items []Event `json:"event"`
}
