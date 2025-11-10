package hook

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

// Delete deletes a hook
func Delete(configFlags *genericclioptions.ConfigFlags, name, namespace string) error {
	c, err := client.GetDynamicClient(configFlags)
	if err != nil {
		return fmt.Errorf("failed to get client: %v", err)
	}

	err = c.Resource(client.HooksGVR).Namespace(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete hook: %v", err)
	}

	fmt.Printf("Hook '%s' deleted from namespace '%s'\n", name, namespace)
	return nil
}
