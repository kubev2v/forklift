package hook

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"

	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

// CreateHookOptions encapsulates the parameters for creating migration hooks.
// This includes the hook name, namespace, configuration flags, and the HookSpec
// containing the hook's operational parameters.
type CreateHookOptions struct {
	Name        string
	Namespace   string
	ConfigFlags *genericclioptions.ConfigFlags
	HookSpec    forkliftv1beta1.HookSpec
}

// Create creates a new migration hook resource.
// It validates the input parameters, encodes the playbook if provided,
// creates the hook resource in Kubernetes, and provides user feedback.
func Create(opts CreateHookOptions) error {
	// Validate the hook specification
	if err := validateHookSpec(opts.HookSpec); err != nil {
		return fmt.Errorf("invalid hook specification: %v", err)
	}

	// Process and encode the playbook if provided
	processedSpec := opts.HookSpec
	if opts.HookSpec.Playbook != "" {
		// If the playbook is not already base64 encoded, encode it
		if !isBase64Encoded(opts.HookSpec.Playbook) {
			encoded := base64.StdEncoding.EncodeToString([]byte(opts.HookSpec.Playbook))
			processedSpec.Playbook = encoded
			klog.V(2).Infof("Encoded playbook content to base64")
		}
	}

	// Create the hook resource
	hookObj, err := createSingleHook(opts.ConfigFlags, opts.Namespace, opts.Name, processedSpec)
	if err != nil {
		return fmt.Errorf("failed to create hook %s: %v", opts.Name, err)
	}

	// Provide user feedback
	fmt.Printf("hook/%s created\n", hookObj.Name)
	klog.V(2).Infof("Created hook '%s' in namespace '%s'", opts.Name, opts.Namespace)

	return nil
}

// validateHookSpec validates the hook specification parameters.
// It ensures that required fields are present and have valid values.
func validateHookSpec(spec forkliftv1beta1.HookSpec) error {
	// Image should not be empty (default is set at command level)
	if spec.Image == "" {
		return fmt.Errorf("image cannot be empty")
	}

	// Validate deadline if provided
	if spec.Deadline < 0 {
		return fmt.Errorf("deadline must be non-negative, got: %d", spec.Deadline)
	}

	return nil
}

// isBase64Encoded checks if a string is already base64 encoded by attempting to decode it.
// This helps avoid double-encoding playbook content.
func isBase64Encoded(s string) bool {
	// Remove any whitespace characters
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\t", "")
	s = strings.ReplaceAll(s, "\r", "")

	// Try to decode the string
	_, err := base64.StdEncoding.DecodeString(s)
	return err == nil && len(s)%4 == 0
}

// createSingleHook creates a single Hook resource in Kubernetes.
// It constructs the Hook object with the provided specification and creates it using the dynamic client.
func createSingleHook(configFlags *genericclioptions.ConfigFlags, namespace, name string, spec forkliftv1beta1.HookSpec) (*forkliftv1beta1.Hook, error) {
	// Create Hook resource
	hookObj := &forkliftv1beta1.Hook{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: spec,
	}

	// Set the API version and kind
	hookObj.Kind = "Hook"
	hookObj.APIVersion = forkliftv1beta1.SchemeGroupVersion.String()

	// Convert to unstructured for dynamic client
	unstructuredHook, err := runtime.DefaultUnstructuredConverter.ToUnstructured(hookObj)
	if err != nil {
		return nil, fmt.Errorf("failed to convert hook to unstructured: %v", err)
	}

	// Get dynamic client
	dynamicClient, err := client.GetDynamicClient(configFlags)
	if err != nil {
		return nil, fmt.Errorf("failed to get dynamic client: %v", err)
	}

	// Create the hook resource
	createdHookUnstructured, err := dynamicClient.Resource(client.HooksGVR).Namespace(namespace).Create(
		context.Background(),
		&unstructured.Unstructured{Object: unstructuredHook},
		metav1.CreateOptions{},
	)

	if err != nil {
		if errors.IsAlreadyExists(err) {
			return nil, fmt.Errorf("hook '%s' already exists in namespace '%s'", name, namespace)
		}
		return nil, fmt.Errorf("failed to create hook: %v", err)
	}

	// Convert back to typed object for return
	var createdHook forkliftv1beta1.Hook
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(createdHookUnstructured.Object, &createdHook)
	if err != nil {
		return nil, fmt.Errorf("failed to convert created hook back to typed object: %v", err)
	}

	return &createdHook, nil
}
