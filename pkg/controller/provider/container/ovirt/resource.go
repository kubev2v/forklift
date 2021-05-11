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
	DataCenter Ref `json:"data_center"`
}

//
// Apply to (update) the model.
func (r *Cluster) ApplyTo(m *model.Cluster) {
	m.Name = r.Name
	m.Name = r.Description
	m.Parent = model.Ref{
		Kind: model.DataCenterKind,
		ID:   r.DataCenter.ID,
	}
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
	Cluster Ref `json:"cluster"`
}

//
// Apply to (update) the model.
func (r *Host) ApplyTo(m *model.Host) {
	m.Name = r.Name
	m.Description = r.Description
	m.Parent = model.Ref{
		Kind: model.ClusterKind,
		ID:   r.Cluster.ID,
	}
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
	Host Ref `json:"host"`
}

//
// Apply to (update) the model.
func (r *VM) ApplyTo(m *model.VM) {
	m.Name = r.Name
	m.Description = r.Description
	m.Parent = model.Ref{
		Kind: model.HostKind,
		ID:   r.Host.ID,
	}
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
}

//
// Apply to (update) the model.
func (r *Network) ApplyTo(m *model.Network) {
	m.Name = r.Name
	m.Description = r.Description
	m.Parent = model.Ref{
		Kind: model.DataCenterKind,
		ID:   r.DataCenter.ID,
	}
	m.VLan = model.Ref{
		Kind: "VLan",
		ID:   r.VLan.ID,
	}
	m.Usages = r.Usages.Usage
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
		m.Parent = model.Ref{
			Kind: model.DataCenterKind,
			ID:   ref.ID,
		}
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
type VNICProfile struct {
	Base
	QoS Ref `json:"qos"`
}

//
// Apply to (update) the model.
func (r *VNICProfile) ApplyTo(m *model.VNICProfile) {
	m.Name = r.Name
	m.Description = r.Description
	m.QoS = model.Ref{
		Kind: "QoS",
		ID:   r.QoS.ID,
	}
}

//
// VNICProfile (list).
type VNICProfileList struct {
	Items []VNICProfile `json:"vnic_profile"`
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
