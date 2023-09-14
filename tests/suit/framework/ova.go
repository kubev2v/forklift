package framework

import (
	"context"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// LoadSourceDetails - Load Source VM details from ova
func (r *OvaClient) LoadSourceDetails() (vm *OvaVM, err error) {
	r.storageClass = DefaultStorageClass
	if sc := os.Getenv("STORAGE_CLASS"); sc != "" {
		r.storageClass = sc
	} else {
		r.storageClass = "nfs-csi"
	}

	r.vmData.testVMId = "21bf790bcdc4591ef01ec6fa7b4812e0d830"
	r.vmData.testNetworkID = "ae1badc8c693926f492a01e2f357d6af321b"
	r.vmData.testStorageName = "Dummy storage for source provider ova-provider"
	return &r.vmData, nil
}

func (r *OvaClient) GetNfsServerForOva(k8sClient *kubernetes.Clientset) (string, error) {
	storageClass, err := k8sClient.StorageV1().StorageClasses().Get(context.TODO(), r.storageClass, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	var nfsShare string
	for parm, val := range storageClass.Parameters {
		if parm == "server" {
			nfsShare = val
		}
		if parm == "share" {
			nfsShare = nfsShare + ":" + val
		}
	}
	if nfsShare != "" {
		r.nfsPath = nfsShare
	}
	return r.nfsPath, nil
}

// GetNetworkId - return the network interface for the VM
func (r *OvaVM) GetNetworkId() string {
	return r.testNetworkID
}

// GetVolumeId - return storage domain IDs
func (r *OvaVM) GetStorageName() string {
	return r.testStorageName
}

// GetTestVMId - return the test VM ID
func (r *OvaVM) GetVmId() string {
	return r.testVMId
}

type OvaClient struct {
	vmData       OvaVM
	CustomEnv    bool
	nfsPath      string
	storageClass string
}

type OvaVM struct {
	testVMId        string
	testNetworkID   string
	testStorageName string
}
