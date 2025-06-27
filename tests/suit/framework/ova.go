package framework

import (
	"context"
	"errors"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// LoadSourceDetails - Load Source VM details from ova
func (r *OvaClient) LoadSourceDetails() (vm *OvaVM, err error) {
	if sc := os.Getenv("STORAGE_CLASS"); sc != "" {
		r.storageClass = sc
	} else {
		r.storageClass = "nfs-csi"
	}

	r.vmData.testVmName = "centos44"
	r.vmData.testNetworkID = "ae1badc8c693926f492a01e2f357d6af321b"
	r.vmData.testStorageName = "centos44_new-disk1.vmdk"
	return &r.vmData, nil
}

func (r *OvaClient) GetNfsServerForOva(k8sClient *kubernetes.Clientset) (string, error) {
	storageClass, err := k8sClient.StorageV1().StorageClasses().Get(context.TODO(), r.storageClass, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	var server, share string
	for parm, val := range storageClass.Parameters {
		if parm == "server" {
			server = val
		}
		if parm == "share" {
			share = val
		}
	}
	nfsShare := server + ":" + share
	if nfsShare == "" {
		return "", errors.New("failed to fetch NFS settings")
	}

	r.nfsPath = nfsShare
	return r.nfsPath, nil
}

// GetNetworkId - return the network interface for the VM
func (r *OvaVM) GetNetworkId() string {
	return r.testNetworkID
}

// GetStorageName - return storage domain IDs
func (r *OvaVM) GetStorageName() string {
	return r.testStorageName
}

// GetVmName - return the test VM name
func (r *OvaVM) GetVmName() string {
	return r.testVmName
}

type OvaClient struct {
	vmData       OvaVM
	CustomEnv    bool
	nfsPath      string
	storageClass string
}

type OvaVM struct {
	testVmName      string
	testNetworkID   string
	testStorageName string
}
