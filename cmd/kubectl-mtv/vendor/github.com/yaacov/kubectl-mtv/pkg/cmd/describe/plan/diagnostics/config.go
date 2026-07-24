package diagnostics

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/get/plan/status"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

// CollectConfigContext gathers migration configuration details relevant for diagnostics.
func CollectConfigContext(ctx context.Context, configFlags *genericclioptions.ConfigFlags, dynClient dynamic.Interface, plan *unstructured.Unstructured) ConfigContext {
	cfg := ConfigContext{}

	cfg.SourceProvider, _, _ = unstructured.NestedString(plan.Object, "spec", "provider", "source", "name")
	cfg.MigrationType = status.GetMigrationType(plan)

	// Try to get VDDK from the source provider's spec
	cfg.VDDKImage = getProviderVDDK(ctx, dynClient, plan)
	if cfg.VDDKImage == "" {
		operatorNS := client.GetMTVOperatorNamespace(ctx, configFlags)
		cfg.VDDKImage = getGlobalVDDK(ctx, dynClient, operatorNS)
	}

	return cfg
}

func getProviderVDDK(ctx context.Context, dynClient dynamic.Interface, plan *unstructured.Unstructured) string {
	providerName, _, _ := unstructured.NestedString(plan.Object, "spec", "provider", "source", "name")
	providerNS, _, _ := unstructured.NestedString(plan.Object, "spec", "provider", "source", "namespace")
	if providerName == "" {
		return ""
	}
	if providerNS == "" {
		providerNS = plan.GetNamespace()
	}

	provider, err := dynClient.Resource(client.ProvidersGVR).Namespace(providerNS).Get(ctx, providerName, metav1.GetOptions{})
	if err != nil {
		return ""
	}

	vddk, _, _ := unstructured.NestedString(provider.Object, "spec", "settings", "vddkInitImage")
	return vddk
}

func getGlobalVDDK(ctx context.Context, dynClient dynamic.Interface, namespace string) string {
	controllers, err := dynClient.Resource(client.ForkliftControllersGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil || len(controllers.Items) == 0 {
		return ""
	}

	for _, ctrl := range controllers.Items {
		vddk, _, _ := unstructured.NestedString(ctrl.Object, "spec", "vddk_image")
		if vddk != "" {
			return vddk
		}
	}
	return ""
}
