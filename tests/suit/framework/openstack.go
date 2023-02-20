package framework

import (
	"fmt"
	"github.com/konveyor/forklift-controller/tests/suit/utils"
)

func (r *OpenStackClient) SetupClient(vmName string, networkName string, volumeTypeName string) (err error) {
	r.vmData.testVMName = vmName
	r.vmData.testNetworkName = networkName
	r.vmData.testVolumeName = volumeTypeName
	return nil
}

// LoadSourceDetails - Load Source VM details from oVirt
func (r *OpenStackClient) LoadSourceDetails(f *Framework, namespace string, contName string) (vm *OpenStackVM, err error) {

	pod, err := utils.FindPodByPrefix(f.K8sClient, namespace, contName, fmt.Sprintf("app=%s", contName))
	if err != nil {
		return nil, fmt.Errorf("error finding Pod for %s - %v", contName, err)
	}

	vmId, err := r.getIdForEntity(f, namespace, pod.Name, contName, "server", r.vmData.testVMName)
	if err != nil {
		return nil, fmt.Errorf("error getting VM ID - %v", err)
	}

	networkId, err := r.getIdForEntity(f, namespace, pod.Name, contName, "network", r.vmData.testNetworkName)
	if err != nil {
		return nil, fmt.Errorf("error getting Network ID - %v", err)
	}

	volumeTypeId, err := r.getIdForEntity(f, namespace, pod.Name, contName, "volume type", r.vmData.testVolumeName)
	if err != nil {
		return nil, fmt.Errorf("error getting volume ID - %v", err)
	}

	r.vmData.testVMId = vmId
	r.vmData.networkId = networkId
	r.vmData.volumeTypeId = volumeTypeId

	return &r.vmData, nil
}

// getIdForEntity - get the ID of the osp entity by given name
func (r *OpenStackClient) getIdForEntity(f *Framework, namespace string, podName string, contName string,
	entType string, entName string) (id string, err error) {
	id, _, err = f.ExecCommandInContainerWithFullOutput(namespace, podName, contName,
		"/bin/bash",
		"-c",
		fmt.Sprintf("source /root/keystonerc_admin && openstack %s show %s -c id -f value", entType, entName))
	return
}

// GetNetworkId - return the network interface for the VM
func (r *OpenStackVM) GetNetworkId() string {
	return r.networkId
}

// GetVolumeId - return storage domain IDs
func (r *OpenStackVM) GetVolumeId() string {
	return r.volumeTypeId
}

// GetTestVMId - return the test VM ID
func (r *OpenStackVM) GetTestVMId() string {
	return r.testVMId
}

// OpenStackClient - OpenStack VM Client
type OpenStackClient struct {
	vmData    OpenStackVM
	CustomEnv bool
}

type OpenStackVM struct {
	networkId       string
	volumeTypeId    string
	testVMId        string
	testVMName      string
	testNetworkName string
	testVolumeName  string
}
