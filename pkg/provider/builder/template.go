package builder

import (
	"context"
	"fmt"

	"github.com/kubev2v/forklift/pkg/templateutil"
	core "k8s.io/api/core/v1"
	cnv "kubevirt.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

// ConfigMapTemplateKey is the key in a ConfigMap that holds the Go text/template content.
const ConfigMapTemplateKey = "template"

// RenderTemplate executes a Go text/template with the given VMBuildValues, producing
// a KubeVirt VirtualMachineSpec. The template must render valid YAML that can be
// unmarshaled into a cnv.VirtualMachineSpec.
//
// The template has access to all sprig string and math functions via templateutil.
func RenderTemplate(templateStr string, values *VMBuildValues) (*cnv.VirtualMachineSpec, error) {
	rendered, err := templateutil.ExecuteTemplate(templateStr, values)
	if err != nil {
		return nil, fmt.Errorf("failed to render VM template: %w", err)
	}

	spec := &cnv.VirtualMachineSpec{}
	if err := yaml.Unmarshal([]byte(rendered), spec); err != nil {
		return nil, fmt.Errorf("failed to unmarshal rendered VM template YAML: %w", err)
	}

	return spec, nil
}

// LoadTemplate reads a Go text/template from a ConfigMap referenced by the given ObjectReference.
// The ConfigMap must contain a key named "template" with the template content.
// Returns the template string or an error if the ConfigMap or key is not found.
func LoadTemplate(k8sClient client.Client, ref *core.ObjectReference) (string, error) {
	if ref == nil {
		return "", fmt.Errorf("template ConfigMap reference is nil")
	}

	configMap := &core.ConfigMap{}
	key := client.ObjectKey{
		Namespace: ref.Namespace,
		Name:      ref.Name,
	}
	if err := k8sClient.Get(context.TODO(), key, configMap); err != nil {
		return "", fmt.Errorf("failed to get template ConfigMap %s/%s: %w", ref.Namespace, ref.Name, err)
	}

	tmpl, ok := configMap.Data[ConfigMapTemplateKey]
	if !ok {
		return "", fmt.Errorf("template ConfigMap %s/%s does not contain key %q", ref.Namespace, ref.Name, ConfigMapTemplateKey)
	}

	if tmpl == "" {
		return "", fmt.Errorf("template ConfigMap %s/%s has empty %q key", ref.Namespace, ref.Name, ConfigMapTemplateKey)
	}

	return tmpl, nil
}
