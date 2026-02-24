package ocp

import (
	"path"
	"strconv"

	net "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/provider/model/base"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
	core "k8s.io/api/core/v1"
	storage "k8s.io/api/storage/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	cnv "kubevirt.io/api/core/v1"
	instancetype "kubevirt.io/api/instancetype/v1beta1"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

// Errors
var NotFound = libmodel.NotFound

type InvalidRefError = base.InvalidRefError

const (
	MaxDetail = base.MaxDetail
)

// Types
type Model = base.Model
type ListOptions = base.ListOptions
type Ref = base.Ref

// k8s Resource.
type Resource interface {
	meta.Object
	runtime.Object
}

// Base k8s model.
type Base struct {
	// Object UID.
	UID string `sql:"pk"`
	// Resource version.
	Version string `sql:"d0"`
	// Namespace.
	Namespace string `sql:"key"`
	// Name.
	Name string `sql:"key"`
	// Labels
	labels libmodel.Labels
}

// Populate fields with the specified k8s resource.
func (m *Base) With(r Resource) {
	m.UID = string(r.GetUID())
	m.Version = r.GetResourceVersion()
	m.Namespace = r.GetNamespace()
	m.Name = r.GetName()
}

// Get kubernetes resource version.
func (m *Base) ResourceVersion() uint64 {
	n, _ := strconv.ParseUint(m.Version, 10, 64)
	return n
}

func (m *Base) Pk() string {
	return m.UID
}

func (m *Base) String() string {
	return path.Join(m.Namespace, m.Name)
}

func (m *Base) Labels() libmodel.Labels {
	return m.labels
}

// Provider
type Provider struct {
	Base
	Type   string       `sql:""`
	Object api.Provider `sql:""`
}

func (m *Provider) With(p *api.Provider) {
	m.Base.With(p)
	m.Type = p.Type().String()
	m.Object = *p
}

// Namespace
type Namespace struct {
	Base
	Object core.Namespace `sql:""`
}

func (m *Namespace) With(n *core.Namespace) {
	m.Base.With(n)
	m.Object = *n
}

// StorageClass
type StorageClass struct {
	Base
	Object storage.StorageClass `sql:""`
}

func (m *StorageClass) With(s *storage.StorageClass) {
	m.Base.With(s)
	m.Object = *s
}

// NetworkAttachmentDefinition
type NetworkAttachmentDefinition struct {
	Base
	Object net.NetworkAttachmentDefinition `sql:""`
}

func (m *NetworkAttachmentDefinition) With(n *net.NetworkAttachmentDefinition) {
	m.Base.With(n)
	m.Object = *n
}

// InstanceTypes
type InstanceType struct {
	Base
	Object instancetype.VirtualMachineInstancetype `sql:""`
}

func (m *InstanceType) With(i *instancetype.VirtualMachineInstancetype) {
	m.Base.With(i)
	m.Object = *i
}

// ClusterInstanceTypes
type ClusterInstanceType struct {
	Base
	Object instancetype.VirtualMachineClusterInstancetype `sql:""`
}

func (m *ClusterInstanceType) With(i *instancetype.VirtualMachineClusterInstancetype) {
	m.Base.With(i)
	m.Object = *i
}

// VM
type VM struct {
	Base
	Object   cnv.VirtualMachine          `sql:""`
	Instance *cnv.VirtualMachineInstance `sql:""`
}

func (m *VM) With(v *cnv.VirtualMachine) {
	m.Base.With(v)
	m.Object = *v
}

func (m *VM) WithVMI(vmi *cnv.VirtualMachineInstance) {
	m.Instance = vmi
}

// PersistentVolumeClaim
type PersistentVolumeClaim struct {
	Base
	Object core.PersistentVolumeClaim `sql:""`
}

func (m *PersistentVolumeClaim) With(pvc *core.PersistentVolumeClaim) {
	m.Base.With(pvc)
	m.Object = *pvc
}

// DataVolume
type DataVolume struct {
	Base
	Object cdi.DataVolume `sql:""`
}

func (m *DataVolume) With(dv *cdi.DataVolume) {
	m.Base.With(dv)
	m.Object = *dv
}

// KubeVirt
type KubeVirt struct {
	Base
	Object cnv.KubeVirt `sql:""`
}

func (m *KubeVirt) With(kv *cnv.KubeVirt) {
	m.Base.With(kv)
	m.Object = *kv
}

type TopologyType string
type RoleType string
type NadType string

// Constants for the supported TopologyType values.
const (
	TopologyLayer2 TopologyType = "layer2"
	TopologyLayer3 TopologyType = "layer3"
	RolePrimary    RoleType     = "primary"
	RoleSecondary  RoleType     = "secondary"
	OvnOverlayType NadType      = "ovn-k8s-cni-overlay"
)

// NetworkConfig represents the structure of the OVN-Kubernetes CNI configuration JSON.
// The `json:"..."` tags are used by the encoding/json package to map the JSON keys
// to the struct fields during marshalling and unmarshalling.
type NetworkConfig struct {
	AllowPersistentIPs bool         `json:"allowPersistentIPs"`
	CNIVersion         string       `json:"cniVersion"`
	JoinSubnet         string       `json:"joinSubnet"`
	Name               string       `json:"name"`
	NetAttachDefName   string       `json:"netAttachDefName"`
	Role               RoleType     `json:"role"`
	Subnets            string       `json:"subnets"`
	Topology           TopologyType `json:"topology"`
	Type               NadType      `json:"type"`
}

func (m *NetworkConfig) IsUnsupportedUdn() bool {
	return m.Type == OvnOverlayType &&
		(m.Role == RolePrimary || m.Topology == TopologyLayer3)
}
