package mapping

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

// Delete deletes a network or storage mapping
func Delete(configFlags *genericclioptions.ConfigFlags, name, namespace, mappingType string) error {
	switch mappingType {
	case "network":
		return DeleteNetwork(configFlags, name, namespace)
	case "storage":
		return DeleteStorage(configFlags, name, namespace)
	default:
		return fmt.Errorf("unsupported mapping type: %s. Use 'network' or 'storage'", mappingType)
	}
}

// DeleteNetwork deletes a network mapping
func DeleteNetwork(configFlags *genericclioptions.ConfigFlags, name, namespace string) error {
	return deleteMapping(configFlags, name, namespace, client.NetworkMapGVR, "network")
}

// DeleteStorage deletes a storage mapping
func DeleteStorage(configFlags *genericclioptions.ConfigFlags, name, namespace string) error {
	return deleteMapping(configFlags, name, namespace, client.StorageMapGVR, "storage")
}

// deleteMapping deletes a mapping resource
func deleteMapping(configFlags *genericclioptions.ConfigFlags, name, namespace string, gvr schema.GroupVersionResource, mappingType string) error {
	c, err := client.GetDynamicClient(configFlags)
	if err != nil {
		return fmt.Errorf("failed to get client: %v", err)
	}

	err = c.Resource(gvr).Namespace(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete %s mapping: %v", mappingType, err)
	}

	fmt.Printf("%s mapping '%s' deleted from namespace '%s'\n",
		fmt.Sprintf("%s%s", string(mappingType[0]-32), mappingType[1:]), name, namespace)
	return nil
}
