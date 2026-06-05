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
	"github.com/yaacov/kubectl-mtv/pkg/util/output"
)

// CreateHookOptions encapsulates the parameters for creating migration hooks.
type CreateHookOptions struct {
	Name             string
	Namespace        string
	ConfigFlags      *genericclioptions.ConfigFlags
	HookSpec         forkliftv1beta1.HookSpec
	DryRun           bool
	OutputFormat     string
	AAPJobTemplateID int
}

// Create creates a new migration hook resource.
func Create(opts CreateHookOptions) error {
	if err := validateHookOptions(opts); err != nil {
		return fmt.Errorf("invalid hook specification: %v", err)
	}

	processedSpec := opts.HookSpec
	if opts.HookSpec.Playbook != "" {
		if !isBase64Encoded(opts.HookSpec.Playbook) {
			encoded := base64.StdEncoding.EncodeToString([]byte(opts.HookSpec.Playbook))
			processedSpec.Playbook = encoded
			klog.V(2).Infof("Encoded playbook content to base64")
		}
	}

	// Build the typed Hook object
	hookObj := &forkliftv1beta1.Hook{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.Name,
			Namespace: opts.Namespace,
		},
		Spec: processedSpec,
	}
	hookObj.Kind = "Hook"
	hookObj.APIVersion = forkliftv1beta1.SchemeGroupVersion.String()

	if opts.DryRun {
		return output.OutputResource(hookObj, opts.OutputFormat)
	}

	// Create the hook resource
	createdHook, err := createSingleHook(opts.ConfigFlags, opts.Namespace, hookObj, opts.AAPJobTemplateID)
	if err != nil {
		return fmt.Errorf("failed to create hook %s: %v", opts.Name, err)
	}

	fmt.Printf("hook/%s created\n", createdHook.GetName())
	klog.V(2).Infof("Created hook '%s' in namespace '%s'", opts.Name, opts.Namespace)

	return nil
}

func validateHookOptions(opts CreateHookOptions) error {
	if opts.AAPJobTemplateID < 0 {
		return fmt.Errorf("AAPJobTemplateID cannot be negative")
	}

	isAAP := opts.AAPJobTemplateID > 0

	if isAAP {
		if opts.HookSpec.Image != "" || opts.HookSpec.Playbook != "" {
			return fmt.Errorf("AAP hooks cannot have image or playbook set")
		}
	} else {
		if opts.HookSpec.Image == "" {
			return fmt.Errorf("image cannot be empty for local hooks")
		}
	}

	if opts.HookSpec.Deadline < 0 {
		return fmt.Errorf("deadline must be non-negative, got: %d", opts.HookSpec.Deadline)
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

// createSingleHook creates a single Hook resource in Kubernetes using the dynamic client.
func createSingleHook(configFlags *genericclioptions.ConfigFlags, namespace string, hookObj *forkliftv1beta1.Hook, aapJobTemplateID int) (*unstructured.Unstructured, error) {
	// Convert to unstructured for dynamic client
	unstructuredHook, err := runtime.DefaultUnstructuredConverter.ToUnstructured(hookObj)
	if err != nil {
		return nil, fmt.Errorf("failed to convert hook to unstructured: %v", err)
	}

	if aapJobTemplateID > 0 {
		if err := unstructured.SetNestedField(unstructuredHook, map[string]interface{}{
			"jobTemplateId": int64(aapJobTemplateID),
		}, "spec", "aap"); err != nil {
			return nil, fmt.Errorf("failed to set AAP config: %v", err)
		}
	}

	dynamicClient, err := client.GetDynamicClient(configFlags)
	if err != nil {
		return nil, fmt.Errorf("failed to get dynamic client: %v", err)
	}

	created, err := dynamicClient.Resource(client.HooksGVR).Namespace(namespace).Create(
		context.Background(),
		&unstructured.Unstructured{Object: unstructuredHook},
		metav1.CreateOptions{},
	)

	if err != nil {
		if errors.IsAlreadyExists(err) {
			return nil, fmt.Errorf("hook '%s' already exists in namespace '%s'", hookObj.Name, namespace)
		}
		return nil, fmt.Errorf("failed to create hook: %v", err)
	}

	return created, nil
}
