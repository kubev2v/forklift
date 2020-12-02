package vsphere

import (
	"encoding/json"
	liberr "github.com/konveyor/controller/pkg/error"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/model/base"
)

//
// Concern severity.
const (
	Advisory = "Advisory"
	Warning  = "Warning"
	Critical = "Critical"
)

//
// Errors
var NotFound = libmodel.NotFound

//
// Types
type Model = libmodel.Model
type Annotation = base.Annotation

//
// Base VMWare model.
type Base struct {
	// Managed object ID.
	ID string `sql:"pk"`
	// Name
	Name string `sql:""`
	// Parent
	Parent string `sql:"index(a)"`
	// Revision
	Revision int64 `sql:""`
}

//
// Get the PK.
func (m *Base) Pk() string {
	return m.ID
}

//
// String representation.
func (m *Base) String() string {
	return m.ID
}

//
// Get labels.
func (m *Base) Labels() libmodel.Labels {
	return nil
}

func (m *Base) Equals(other libmodel.Model) bool {
	if vm, cast := other.(*VM); cast {
		return m.ID == vm.ID
	}

	return false
}

//
// Populate PK using the ref.
func (m *Base) WithRef(ref Ref) {
	m.ID = ref.ID
}

//
// Created.
func (m *Base) Created() {
	m.Revision = 1
}

//
// Updated.
// Increment revision. Should ONLY be called by
// the reconciler.
func (m *Base) Updated() {
	m.Revision++
}

//
// An object reference.
type Ref struct {
	// The kind (type) of the referenced.
	Kind string `json:"kind"`
	// The ID of object referenced.
	ID string `json:"id"`
}

//
// Encode the ref.
func (r *Ref) Encode() string {
	j, _ := json.Marshal(r)
	return string(j)
}

//
// Unmarshal the json `j` into self.
func (r *Ref) With(j string) *Ref {
	json.Unmarshal([]byte(j), r)
	return r
}

//
// Ref pointer.
func RefPtr() *Ref {
	r := Ref{}
	return &r
}

//
// List of `Ref`.
type RefList []Ref

//
// Encode the list.
func (r *RefList) Encode() string {
	j, _ := json.Marshal(r)
	return string(j)
}

//
// Unmarshal the json `j` into self.
func (r *RefList) With(j string) *RefList {
	json.Unmarshal([]byte(j), r)
	return r
}

//
// RefList pointer.
func RefListPtr() *RefList {
	r := RefList{}
	return &r
}

//
// List
type List []interface{}

func (r *List) Encode() string {
	j, _ := json.Marshal(r)
	return string(j)
}

//
// Unmarshal the json `j` into self.
func (r *List) With(j string) *List {
	json.Unmarshal([]byte(j), r)
	return r
}

type About struct {
	Base
	APIVersion string `sql:""`
	Product    string `sql:""`
}

type Folder struct {
	Base
	Children string `sql:""`
}

type Datacenter struct {
	Base
	Clusters   string `sql:""`
	Networks   string `sql:""`
	Datastores string `sql:""`
	Vms        string `sql:""`
}

type Cluster struct {
	Base
	Hosts       string `sql:""`
	Networks    string `sql:""`
	Datastores  string `sql:""`
	DasEnabled  bool   `sql:""`
	DasVms      string `sql:""`
	DrsEnabled  bool   `sql:""`
	DrsBehavior string `sql:""`
	DrsVms      string `sql:""`
}

type Host struct {
	Base
	InMaintenanceMode bool   `sql:""`
	Thumbprint        string `sql:""`
	CpuSockets        int16  `sql:""`
	CpuCores          int16  `sql:""`
	ProductName       string `sql:""`
	ProductVersion    string `sql:""`
	Network           string `sql:""`
	Networks          string `sql:""`
	Datastores        string `sql:""`
	Vms               string `sql:""`
}

func (r *Host) EncodeNetwork(network *HostNetwork) {
	b, _ := json.Marshal(network)
	r.Network = string(b)
}

func (r *Host) DecodeNetwork() *HostNetwork {
	network := &HostNetwork{}
	json.Unmarshal([]byte(r.Network), network)
	return network
}

type HostNetwork struct {
	PNICs      []PNIC      `json:"vNICs"`
	VNICs      []VNIC      `json:"pNICs"`
	PortGroups []PortGroup `json:"portGroups"`
	Switches   []Switch    `json:"switches"`
}

func (n *HostNetwork) Switch(key string) (vSwitch *Switch, found bool) {
	for _, object := range n.Switches {
		if key == object.Key {
			vSwitch = &object
			found = true
			break
		}
	}

	return
}
func (n *HostNetwork) PortGroup(name string) (portGroup *PortGroup, found bool) {
	for _, object := range n.PortGroups {
		if name == object.Name {
			portGroup = &object
			found = true
			break
		}
	}

	return
}

func (n *HostNetwork) PNIC(key string) (nic *PNIC, found bool) {
	for _, object := range n.PNICs {
		if key == object.Key {
			nic = &object
			found = true
			break
		}
	}

	return
}

type PNIC struct {
	Key       string `json:"key"`
	LinkSpeed int32  `json:"linkSpeed"`
}

type VNIC struct {
	Key        string `json:"key"`
	PortGroup  string `json:"portGroup"`
	DPortGroup string `json:"dPortGroup"`
	IpAddress  string `json:"ipAddress"`
	MTU        int32  `json:"mtu"`
}

type PortGroup struct {
	Key    string `json:"key"`
	Name   string `json:"name"`
	Switch string `json:"vSwitch"`
}

type Switch struct {
	Key        string   `json:"key"`
	Name       string   `json:"name"`
	PortGroups []string `json:"portGroups"`
	PNICs      []string `json:"pNICs"`
}

type Network struct {
	Base
	Tag      string `sql:""`
	DVSwitch string `sql:""`
}

type DVSwitch struct {
	Base
	Host string `sql:""`
}

type DVSHost struct {
	Host string
	PNIC []string
}

func (m *DVSwitch) EncodeHost(host []DVSHost) {
	j, _ := json.Marshal(host)
	m.Host = string(j)
}

func (m *DVSwitch) DecodeHost() []DVSHost {
	list := []DVSHost{}
	json.Unmarshal([]byte(m.Host), &list)
	return list
}

type Datastore struct {
	Base
	Type            string `sql:""`
	Capacity        int64  `sql:""`
	Free            int64  `sql:""`
	MaintenanceMode string `sql:""`
}

type VM struct {
	Base
	UUID                  string `sql:""`
	Firmware              string `sql:""`
	CpuAffinity           string `sql:""`
	CpuHotAddEnabled      bool   `sql:""`
	CpuHotRemoveEnabled   bool   `sql:""`
	MemoryHotAddEnabled   bool   `sql:""`
	FaultToleranceEnabled bool   `sql:""`
	CpuCount              int32  `sql:""`
	CoresPerSocket        int32  `sql:""`
	MemoryMB              int32  `sql:""`
	GuestName             string `sql:""`
	BalloonedMemory       int32  `sql:""`
	IpAddress             string `sql:""`
	NumaNodeAffinity      string `sql:""`
	StorageUsed           int64  `sql:""`
	SriovSupported        bool   `sql:""`
	PassthroughSupported  bool   `sql:""`
	UsbSupported          bool   `sql:""`
	Disks                 string `sql:""`
	Networks              string `sql:""`
	Host                  string `sql:""`
	RevisionAnalyzed      int64  `sql:""`
	Concerns              string `sql:""`
}

//
// Determine if current revision has been analyzed.
func (m *VM) Analyzed() bool {
	return m.RevisionAnalyzed == m.Revision
}

//
// Encode disks.
func (m *VM) EncodeDisks(d []Disk) {
	j, _ := json.Marshal(d)
	m.Disks = string(j)
}

//
// Decode disks.
func (m *VM) DecodeDisks() []Disk {
	list := []Disk{}
	json.Unmarshal([]byte(m.Disks), &list)
	return list
}

//
// Find associated cluster.
// returns err = libmodel.NotFound when cannot be resolved.
func (m *VM) Cluster(db libmodel.DB) (matched *Cluster, err error) {
	list := []Cluster{}
	err = db.List(&list, libmodel.ListOptions{})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	hostRef := Ref{}
	hostRef.With(m.Host)
	for _, cluster := range list {
		refList := RefList{}
		refList.With(cluster.Hosts)
		for _, ref := range refList {
			if ref.ID == hostRef.ID {
				matched = &cluster
				return
			}
		}
	}

	err = liberr.Wrap(libmodel.NotFound)

	return
}

//
// Virtual Disk.
type Disk struct {
	File      string `json:"file"`
	Datastore Ref    `json:"datastore"`
	Capacity  int64  `json:"capacity"`
	Shared    bool   `json:"shared"`
	RDM       bool   `json:"rdm"`
}

//
// VM concerns.
type Concern struct {
	Name     string `json:"name"`
	Severity string `json:"severity"`
}
