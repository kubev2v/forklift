package version

import (
	"context"

	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// Info holds all version-related information
type Info struct {
	ClientVersion     string `json:"clientVersion" yaml:"clientVersion"`
	OperatorVersion   string `json:"operatorVersion" yaml:"operatorVersion"`
	OperatorStatus    string `json:"operatorStatus" yaml:"operatorStatus"`
	OperatorNamespace string `json:"operatorNamespace,omitempty" yaml:"operatorNamespace,omitempty"`
	InventoryURL      string `json:"inventoryURL" yaml:"inventoryURL"`
	InventoryStatus   string `json:"inventoryStatus" yaml:"inventoryStatus"`
}

// GetInventoryInfo returns information about the MTV inventory service
func GetInventoryInfo(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags) (string, string) {
	namespace := client.ResolveNamespace(kubeConfigFlags)

	// Try to discover inventory URL
	inventoryURL := client.DiscoverInventoryURL(ctx, kubeConfigFlags, namespace)
	if inventoryURL != "" {
		return inventoryURL, "available"
	}

	return "not found", "not available"
}

// GetMTVControllerInfo returns information about the MTV Operator
func GetMTVControllerInfo(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags) (string, string, string) {
	operatorInfo := client.GetMTVOperatorInfo(ctx, kubeConfigFlags)

	if !operatorInfo.Found {
		return "not found", "not available", ""
	}

	status := "installed"
	version := operatorInfo.Version
	namespace := operatorInfo.Namespace

	if namespace == "" {
		namespace = "unknown"
	}

	return version, status, namespace
}

// GetVersionInfo gathers all version information
func GetVersionInfo(ctx context.Context, clientVersion string, kubeConfigFlags *genericclioptions.ConfigFlags) Info {
	// Get MTV Operator information
	controllerVersion, controllerStatus, controllerNamespace := GetMTVControllerInfo(ctx, kubeConfigFlags)

	// Get inventory information
	inventoryURL, inventoryStatus := GetInventoryInfo(ctx, kubeConfigFlags)

	return Info{
		ClientVersion:     clientVersion,
		OperatorVersion:   controllerVersion,
		OperatorStatus:    controllerStatus,
		OperatorNamespace: controllerNamespace,
		InventoryURL:      inventoryURL,
		InventoryStatus:   inventoryStatus,
	}
}
