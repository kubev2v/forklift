package client

import (
	"context"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// MTVOperatorInfo holds information about the MTV Operator installation
type MTVOperatorInfo struct {
	Version   string
	Namespace string
	Found     bool
}

// GetMTVOperatorInfo discovers information about the MTV Operator installation
// by examining the providers.forklift.konveyor.io CRD annotations.
// Returns operator version, namespace, and whether the operator was found.
func GetMTVOperatorInfo(ctx context.Context, configFlags *genericclioptions.ConfigFlags) MTVOperatorInfo {
	info := MTVOperatorInfo{
		Version:   "unknown",
		Namespace: "",
		Found:     false,
	}

	// Try to get dynamic client
	dynamicClient, err := GetDynamicClient(configFlags)
	if err != nil {
		return info
	}

	// Check if MTV is installed by looking for the providers CRD
	crdGVR := schema.GroupVersionResource{
		Group:    "apiextensions.k8s.io",
		Version:  "v1",
		Resource: "customresourcedefinitions",
	}

	crd, err := dynamicClient.Resource(crdGVR).Get(ctx, "providers.forklift.konveyor.io", metav1.GetOptions{})
	if err != nil {
		return info
	}

	info.Found = true

	// Try to get version and namespace from operator annotation
	if annotations, found, _ := unstructured.NestedStringMap(crd.Object, "metadata", "annotations"); found {
		// Look for operatorframework.io/installed-alongside annotation
		for key, value := range annotations {
			if strings.HasPrefix(key, "operatorframework.io/installed-alongside-") {
				// Format: namespace/operator-name.version
				parts := strings.Split(value, "/")
				if len(parts) == 2 {
					info.Namespace = parts[0]
					info.Version = parts[1]
				}
				break
			}
		}
	}

	return info
}

// GetMTVOperatorNamespace returns the namespace where MTV operator is installed.
// It first tries to discover it from CRD annotations, then falls back to the
// hardcoded OpenShiftMTVNamespace constant if not found.
func GetMTVOperatorNamespace(ctx context.Context, configFlags *genericclioptions.ConfigFlags) string {
	operatorInfo := GetMTVOperatorInfo(ctx, configFlags)
	if operatorInfo.Found && operatorInfo.Namespace != "" {
		return operatorInfo.Namespace
	}
	// Fall back to hardcoded default
	return OpenShiftMTVNamespace
}
