package ovirt

import (
	"sort"
	"strconv"

	model "github.com/kubev2v/forklift/pkg/controller/provider/model/ovirt"
)

// System.
type System struct {
	Product struct {
		Name    string `json:"name"`
		Vendor  string `json:"vendor"`
		Version struct {
			FullVersion string `json:"full_version"`
		} `json:"version"`
	} `json:"product_info"`
}

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

func (b *Base) bool(s string) (v bool) {
	v, _ = strconv.ParseBool(s)
	return
}

func (b *Base) int16(s string) (v int16) {
	n, _ := strconv.ParseInt(s, 10, 16)
	v = int16(n)
	return
}

func (b *Base) int32(s string) (v int32) {
	n, _ := strconv.ParseInt(s, 10, 32)
	v = int32(n)
	return
}

func (b *Base) int64(s string) (v int64) {
	v, _ = strconv.ParseInt(s, 10, 64)
	return
}

// DataCenter.
type DataCenter struct {
	Base
}

// Apply to (update) the model.
func (r *DataCenter) ApplyTo(m *model.DataCenter) {
	m.Name = r.Name
	m.Description = r.Description
}

// DataCenter (list).
type DataCenterList struct {
	Items []DataCenter `json:"data_center"`
}

// Cluster.
type Cluster struct {
	Base
	DataCenter    Ref    `json:"data_center"`
	HaReservation string `json:"ha_reservation"`
	KSM           struct {
		Enabled string `json:"enabled"`
	} `json:"ksm"`
	BiosType string `json:"bios_type"`
	CPU      struct {
		Type string `json:"type"`
	} `json:"cpu"`
	Version struct {
		Minor string `json:"minor"`
		Major string `json:"major"`
	} `json:"version"`
}

// Apply to (update) the model.
func (r *Cluster) ApplyTo(m *model.Cluster) {
	m.Name = r.Name
	m.Description = r.Description
	m.DataCenter = r.DataCenter.ID
	m.HaReservation = r.bool(r.HaReservation)
	m.KsmEnabled = r.bool(r.KSM.Enabled)
	m.BiosType = r.BiosType
	m.CPU.Type = r.CPU.Type
	m.Version.Minor = r.Version.Minor
	m.Version.Major = r.Version.Major
}

// Cluster (list).
type ClusterList struct {
	Items []Cluster `json:"cluster"`
}

// ServerCpu.
type ServerCpu struct {
	Base
	Values struct {
		SystemOptionValues []SystemOptionValue `json:"system_option_value"`
	} `json:"values"`
}

type SystemOptionValue struct {
	Value   string `json:"value"`
	Version string `json:"version"`
}

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
	SSH struct {
		Thumbprint string `json:"thumbprint"`
	} `json:"ssh"`
	NICs struct {
		List []struct {
			ID        string `json:"id"`
			Name      string `json:"name"`
			LinkSpeed string `json:"speed"`
			MTU       string `json:"mtu"`
			VLan      struct {
				ID string `json:"id"`
			} `json:"vlan"`
		} `json:"host_nic"`
	} `json:"nics"`
	Networks struct {
		Attachment []struct {
			ID      string `json:"id"`
			Network Ref    `json:"network"`
		} `json:"network_attachment"`
	} `json:"network_attachments"`
}

// Apply to (update) the model.
func (r *Host) ApplyTo(m *model.Host) {
	m.Name = r.Name
	m.Description = r.Description
	m.Cluster = r.Cluster.ID
	m.Status = r.Status
	m.ProductName = r.OS.Type
	m.ProductVersion = r.OS.Version.Full
	m.InMaintenance = r.Status == "maintenance"
	m.CpuSockets = r.int16(r.CPU.Topology.Sockets)
	m.CpuCores = r.int16(r.CPU.Topology.Cores)
	r.addNetworkAttachment(m)
	r.addNICs(m)
}

func (r *Host) addNetworkAttachment(m *model.Host) {
	m.NetworkAttachments = []model.NetworkAttachment{}
	for _, n := range r.Networks.Attachment {
		m.NetworkAttachments = append(
			m.NetworkAttachments,
			model.NetworkAttachment{
				ID:      n.ID,
				Network: n.Network.ID,
			})
	}
}

func (r *Host) addNICs(m *model.Host) {
	m.NICs = []model.HostNIC{}
	for _, n := range r.NICs.List {
		m.NICs = append(
			m.NICs,
			model.HostNIC{
				ID:        n.ID,
				Name:      n.Name,
				LinkSpeed: r.int64(n.LinkSpeed),
				MTU:       r.int64(n.MTU),
				VLan:      n.VLan.ID,
			})
	}
}

// Host (list).
type HostList struct {
	Items []Host `json:"host"`
}

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
	OS struct {
		Type    string `json:"type"`
		Version struct {
			Full string `json:"full_version"`
		} `json:"os"`
	}
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
			Threads string `json:"threads"`
		} `json:"topology"`
	} `json:"cpu"`
	CpuPinningPolicy string `json:"cpu_pinning_policy"`
	CpuShares        string `json:"cpu_shares"`
	USB              struct {
		Enabled string `json:"enabled"`
	} `json:"usb"`
	Timezone struct {
		Name string `json:"name"`
	} `json:"time_zone"`
	Status       string `json:"status"`
	Stateless    string `json:"stateless"`
	SerialNumber struct {
		Value string `json:"value"`
	} `json:"serial_number"`
	PlacementPolicy struct {
		Affinity string `json:"affinity"`
	} `json:"placement_policy"`
	Memory string `json:"memory"`
	IO     struct {
		Threads string `json:"threads"`
	} `json:"io"`
	BIOS struct {
		Type     string `json:"type"`
		BootMenu struct {
			Enabled string `json:"enabled"`
		} `json:"boot_menu"`
	} `json:"bios"`
	CustomCpuModel string `json:"custom_cpu_model"`
	Display        struct {
		Type string `json:"type"`
	} `json:"display"`
	HasIllegalImages string `json:"has_illegal_images"`
	Lease            struct {
		StorageDomain Ref `json:"storage_domain"`
	} `json:"lease"`
	StorageErrorResumeBehaviour string `json:"storage_error_resume_behaviour"`
	MemoryPolicy                struct {
		Ballooning string `json:"ballooning"`
	} `json:"memory_policy"`
	HA struct {
		Enabled string `json:"enabled"`
	} `json:"high_availability"`
	HostDevices struct {
		List []struct {
			Capability string `json:"capability"`
			Vendor     struct {
				Name string `json:"name"`
			} `json:"vendor"`
			Product struct {
				Name string `json:"name"`
			} `json:"product"`
		} `json:"host_device"`
	} `json:"host_devices"`
	CDROMs struct {
		List []struct {
			ID   string `json:"id"`
			File struct {
				ID string `json:"id"`
			} `json:"file"`
		} `json:"cdrom"`
	} `json:"cdroms"`
	NICs struct {
		List []struct {
			ID        string `json:"id"`
			Name      string `json:"name"`
			Interface string `json:"interface"`
			MAC       struct {
				Address string `json:"address"`
			} `json:"mac"`
			Plugged string `json:"plugged"`
			Profile Ref    `json:"vnic_profile"`
			Devices struct {
				List []struct {
					IPS struct {
						IP []struct {
							Address string `json:"address"`
							Version string `json:"version"`
						} `json:"ip"`
					} `json:"ips"`
				} `json:"reported_device"`
			} `json:"reported_devices"`
		} `json:"nic"`
	} `json:"nics"`
	Disks struct {
		Attachment []struct {
			ID              string `json:"id"`
			Bootable        string `json:"bootable"`
			Name            string
			Interface       string `json:"interface"`
			SCSIReservation string `json:"uses_scsi_reservation"`
			Disk            Ref    `json:"disk"`
		} `json:"disk_attachment"`
	} `json:"disk_attachments"`
	WatchDogs struct {
		List []struct {
			ID     string `json:"id"`
			Action string `json:"action"`
			Model  string `json:"model"`
		} `json:"watchdog"`
	} `json:"watchdogs"`
	Properties struct {
		List []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"custom_property"`
	} `json:"custom_properties"`
	Snapshots struct {
		List []struct {
			ID            string `json:"id"`
			Description   string `json:"description"`
			PersistMemory string `json:"persist_memorystate"`
			Type          string `json:"snapshot_type"`
		} `json:"snapshot"`
	} `json:"snapshots"`
}

// Apply to (update) the model.
func (r *VM) ApplyTo(m *model.VM) {
	m.Name = r.Name
	m.Description = r.Description
	m.Cluster = r.Cluster.ID
	m.Host = r.Host.ID
	m.GuestName = r.Guest.Distribution + " " + r.Guest.Version.Full
	m.Guest.Distribution = r.Guest.Distribution
	m.Guest.FullVersion = r.Guest.Version.Full
	m.OSType = r.OS.Type
	m.CpuSockets = r.int16(r.CPU.Topology.Sockets)
	m.CpuCores = r.int16(r.CPU.Topology.Cores)
	m.CpuThreads = r.int16(r.CPU.Topology.Threads)
	m.CpuPinningPolicy = r.CpuPinningPolicy
	m.CpuShares = r.int16(r.CpuShares)
	m.Memory = r.int64(r.Memory)
	m.BIOS = r.BIOS.Type
	m.UsbEnabled = r.bool(r.USB.Enabled)
	m.BootMenuEnabled = r.bool(r.BIOS.BootMenu.Enabled)
	m.PlacementPolicyAffinity = r.PlacementPolicy.Affinity
	m.Timezone = r.Timezone.Name
	m.Status = r.Status
	m.Stateless = r.Stateless
	m.SerialNumber = r.SerialNumber.Value
	m.Display = r.Display.Type
	m.HasIllegalImages = r.bool(r.HasIllegalImages)
	m.BalloonedMemory = r.bool(r.MemoryPolicy.Ballooning)
	m.NumaNodeAffinity = []string{}
	m.LeaseStorageDomain = r.Lease.StorageDomain.ID
	m.StorageErrorResumeBehaviour = r.StorageErrorResumeBehaviour
	m.HaEnabled = r.bool(r.HA.Enabled)
	m.IOThreads = r.int16(r.IO.Threads)
	m.CustomCpuModel = r.CustomCpuModel
	r.addCpuAffinity(m)
	r.addNICs(m)
	r.addDiskAttachment(m)
	r.addHostDevices(m)
	r.addCDROMs(m)
	r.addWatchDogs(m)
	r.addProperties(m)
	r.addSnapshot(m)
}

func (r *VM) addCpuAffinity(m *model.VM) {
	m.CpuAffinity = []model.CpuPinning{}
	for _, p := range r.CPU.Tune.Pin.List {
		m.CpuAffinity = append(
			m.CpuAffinity, model.CpuPinning{
				Set: r.int32(p.Set),
				Cpu: r.int32(p.Cpu),
			})
	}
}

func (r *VM) addNICs(m *model.VM) {
	m.NICs = []model.NIC{}
	for _, n := range r.NICs.List {
		ips := []model.IpAddress{}
		for _, d := range n.Devices.List {
			for _, ip := range d.IPS.IP {
				ips = append(
					ips,
					model.IpAddress{
						Address: ip.Address,
						Version: ip.Version,
					})
			}
		}
		m.NICs = append(
			m.NICs, model.NIC{
				ID:        n.ID,
				Name:      n.Name,
				Profile:   n.Profile.ID,
				Interface: n.Interface,
				MAC:       n.MAC.Address,
				Plugged:   r.bool(n.Plugged),
				IpAddress: ips,
			})
	}
}

func (r *VM) addDiskAttachment(m *model.VM) {
	m.DiskAttachments = []model.DiskAttachment{}
	for _, da := range r.Disks.Attachment {
		m.DiskAttachments = append(
			m.DiskAttachments,
			model.DiskAttachment{
				ID:              da.ID,
				Interface:       da.Interface,
				SCSIReservation: r.bool(da.SCSIReservation),
				Disk:            da.Disk.ID,
				Bootable:        r.bool(da.Bootable),
			})
	}
}

func (r *VM) addHostDevices(m *model.VM) {
	m.HostDevices = []model.HostDevice{}
	for _, d := range r.HostDevices.List {
		m.HostDevices = append(
			m.HostDevices,
			model.HostDevice{
				Capability: d.Capability,
				Product:    d.Product.Name,
				Vendor:     d.Vendor.Name,
			})
	}
}

func (r *VM) addCDROMs(m *model.VM) {
	m.CDROMs = []model.CDROM{}
	for _, cd := range r.CDROMs.List {
		m.CDROMs = append(
			m.CDROMs,
			model.CDROM{
				ID:   cd.ID,
				File: cd.File.ID,
			})
	}
}

func (r *VM) addWatchDogs(m *model.VM) {
	m.WatchDogs = []model.WatchDog{}
	for _, w := range r.WatchDogs.List {
		m.WatchDogs = append(
			m.WatchDogs,
			model.WatchDog{
				ID:     w.ID,
				Action: w.Action,
				Model:  w.Model,
			})
	}
}

func (r *VM) addProperties(m *model.VM) {
	m.Properties = []model.Property{}
	for _, p := range r.Properties.List {
		m.Properties = append(
			m.Properties,
			model.Property{
				Name:  p.Name,
				Value: p.Value,
			})
	}
}

func (r *VM) addSnapshot(m *model.VM) {
	m.Snapshots = []model.Snapshot{}
	for _, sn := range r.Snapshots.List {
		m.Snapshots = append(
			m.Snapshots,
			model.Snapshot{
				ID:            sn.ID,
				Description:   sn.Description,
				PersistMemory: r.bool(sn.PersistMemory),
				Type:          sn.Type,
			})
	}
}

// VM (list).
type VMList struct {
	Items []VM `json:"vm"`
}

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

// Apply to (update) the model.
func (r *Network) ApplyTo(m *model.Network) {
	m.Name = r.Name
	m.Description = r.Description
	m.DataCenter = r.DataCenter.ID
	m.VLan = r.VLan.ID
	m.Usages = r.Usages.Usage
	r.setProfiles(m)
}

func (r *Network) setProfiles(m *model.Network) {
	m.Profiles = []string{}
	for _, p := range r.Profiles.List {
		m.Profiles = append(m.Profiles, p.ID)
	}
}

// Network (list).
type NetworkList struct {
	Items []Network `json:"network"`
}

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

// Apply to (update) the model.
func (r *StorageDomain) ApplyTo(m *model.StorageDomain) {
	m.Name = r.Name
	m.Description = r.Description
	m.Type = r.Type
	m.Storage.Type = r.Storage.Type
	m.Available = r.int64(r.Available)
	m.Used = r.int64(r.Used)
	r.setDataCenter(m)
}

func (r *StorageDomain) setDataCenter(m *model.StorageDomain) {
	for _, ref := range r.DataCenter.List {
		m.DataCenter = ref.ID
		break
	}
}

// StorageDomain (list).
type StorageDomainList struct {
	Items []StorageDomain `json:"storage_domain"`
}

// vNIC profile.
type NICProfile struct {
	Base
	Network       Ref    `json:"network"`
	QoS           Ref    `json:"qos"`
	NetworkFilter Ref    `json:"network_filter"`
	PortMirroring string `json:"port_mirroring"`
	Properties    struct {
		List []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"custom_property"`
	} `json:"custom_properties"`
	PassThrough struct {
		Mode string `json:"mode"`
	} `json:"pass_through"`
}

// Apply to (update) the model.
func (r *NICProfile) ApplyTo(m *model.NICProfile) {
	m.Name = r.Name
	m.Description = r.Description
	m.Network = r.Network.ID
	m.NetworkFilter = r.NetworkFilter.ID
	m.PortMirroring = r.bool(r.PortMirroring)
	m.PassThrough = r.PassThrough.Mode == "enabled"
	m.QoS = r.QoS.ID
	r.addProperties(m)
}

func (r *NICProfile) addProperties(m *model.NICProfile) {
	properties := []model.Property{}
	for _, p := range r.Properties.List {
		properties = append(
			properties,
			model.Property{
				Name:  p.Name,
				Value: p.Value,
			})
	}
	m.Properties = properties
}

// NICProfile (list).
type NICProfileList struct {
	Items []NICProfile `json:"vnic_profile"`
}

// Disk profile.
type DiskProfile struct {
	Base
	StorageDomain Ref `json:"storage_domain"`
	QoS           Ref `json:"qos"`
}

// Apply to (update) the model.
func (r *DiskProfile) ApplyTo(m *model.DiskProfile) {
	m.Name = r.Name
	m.Description = r.Description
	m.StorageDomain = r.StorageDomain.ID
	m.QoS = r.QoS.ID
}

// NICProfile (list).
type DiskProfileList struct {
	Items []DiskProfile `json:"disk_profile"`
}

// Disk.
type Disk struct {
	Base
	Sharable        string `json:"sharable"`
	Profile         Ref    `json:"disk_profile"`
	ProvisionedSize string `json:"provisioned_size"`
	StorageDomains  struct {
		List []Ref `json:"storage_domain"`
	} `json:"storage_domains"`
	Status      string `json:"status"`
	ActualSize  string `json:"actual_size"`
	Backup      string `json:"backup"`
	StorageType string `json:"storage_type"`
	Lun         Lun    `json:"lun_storage"`
}

// LUN Resource.
type Lun struct {
	LogicalUnits struct {
		LogicalUnit []LogicalUnit `json:"logical_unit"`
	} `json:"logical_units"`
}

type LogicalUnit struct {
	Base
	Address    string `json:"address"`
	Port       string `json:"port"`
	Target     string `json:"target"`
	LunMapping string `json:"lun_mapping"`
	Size       string `json:"size"`
}

// Apply to (update) the model.
func (r *Disk) ApplyTo(m *model.Disk) {
	m.Name = r.Name
	m.Description = r.Description
	m.Shared = r.bool(r.Sharable)
	m.Profile = r.Profile.ID
	m.Status = r.Status
	m.ActualSize = r.int64(r.ActualSize)
	m.Backup = r.Backup
	m.StorageType = r.StorageType
	m.ProvisionedSize = r.int64(r.ProvisionedSize)
	r.setStorageDomain(m)
	r.setLun(m)
}

func (r *Disk) setStorageDomain(m *model.Disk) {
	for _, ref := range r.StorageDomains.List {
		m.StorageDomain = ref.ID
		break
	}
}

func (r *Disk) setLun(m *model.Disk) {
	m.Lun = model.Lun{}
	m.Lun.LogicalUnits.LogicalUnit = []model.LogicalUnit{}
	for _, rlu := range r.Lun.LogicalUnits.LogicalUnit {
		mlu := &model.LogicalUnit{}
		rlu.ApplyTo(mlu)
		m.Lun.LogicalUnits.LogicalUnit = append(m.Lun.LogicalUnits.LogicalUnit, *mlu)
	}
}

func (r *LogicalUnit) ApplyTo(m *model.LogicalUnit) {
	m.LunID = r.ID
	m.Address = r.Address
	m.Port = r.Port
	m.Target = r.Target
	m.LunMapping = r.int32(r.LunMapping)
	m.Size = r.int64(r.Size)
}

// Disk (list).
type DiskList struct {
	Items []Disk `json:"disk"`
}

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

// EVent (list).
type EventList struct {
	Items []Event `json:"event"`
}

// Sort by ID ascending.
func (r *EventList) sort() {
	sort.Slice(
		r.Items,
		func(i, j int) bool {
			return r.Items[i].id() < r.Items[j].id()
		})
}
