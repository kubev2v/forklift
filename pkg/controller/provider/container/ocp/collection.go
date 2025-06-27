package ocp

import (
	net "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	libocp "github.com/kubev2v/forklift/pkg/lib/inventory/container/ocp"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	core "k8s.io/api/core/v1"
	storage "k8s.io/api/storage/v1"
	cnv "kubevirt.io/api/core/v1"
	instancetype "kubevirt.io/api/instancetype/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// StorageClass
type StorageClass struct {
	libocp.BaseCollection
	log logging.LevelLogger
}

// Get the kubernetes object being collected.
func (r *StorageClass) Object() client.Object {
	return &storage.StorageClass{}
}

// NetworkAttachmentDefinition
type NetworkAttachmentDefinition struct {
	libocp.BaseCollection
	log logging.LevelLogger
}

// Get the kubernetes object being collected.
func (r *NetworkAttachmentDefinition) Object() client.Object {
	return &net.NetworkAttachmentDefinition{}
}

// Namespace
type Namespace struct {
	libocp.BaseCollection
	log logging.LevelLogger
}

// Get the kubernetes object being collected.
func (r *Namespace) Object() client.Object {
	return &core.Namespace{}
}

// VM
type VM struct {
	libocp.BaseCollection
	log logging.LevelLogger
}

// Get the kubernetes object being collected.
func (r *VM) Object() client.Object {
	return &cnv.VirtualMachine{}
}

// InstanceType
type InstanceType struct {
	libocp.BaseCollection
	log logging.LevelLogger
}

// Get the kubernetes object being collected.
func (r *InstanceType) Object() client.Object {
	return &instancetype.VirtualMachineInstancetype{}
}

// ClusterInstanceType
type ClusterInstanceType struct {
	libocp.BaseCollection
	log logging.LevelLogger
}

// Get the kubernetes object being collected.
func (r *ClusterInstanceType) Object() client.Object {
	return &instancetype.VirtualMachineClusterInstancetype{}
}
