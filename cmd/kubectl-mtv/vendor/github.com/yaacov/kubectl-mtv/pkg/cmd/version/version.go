package version

import (
	"context"

	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/config"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// Info holds all version-related information
type Info struct {
	ClientVersion     string `json:"clientVersion" yaml:"clientVersion"`
	OperatorVersion   string `json:"operatorVersion,omitempty" yaml:"operatorVersion,omitempty"`
	OperatorStatus    string `json:"operatorStatus,omitempty" yaml:"operatorStatus,omitempty"`
	OperatorNamespace string `json:"operatorNamespace,omitempty" yaml:"operatorNamespace,omitempty"`
	InventoryURL      string `json:"inventoryURL,omitempty" yaml:"inventoryURL,omitempty"`
	InventoryStatus   string `json:"inventoryStatus,omitempty" yaml:"inventoryStatus,omitempty"`
	InventoryInsecure bool   `json:"inventoryInsecure,omitempty" yaml:"inventoryInsecure,omitempty"`
}

// GetInventoryInfo returns information about the MTV inventory service
// Uses the global config which already handles:
// - Checking MTV_INVENTORY_URL env var
// - Auto-discovering from OpenShift routes
// - Caching the result
func GetInventoryInfo(globalConfig config.InventoryConfigGetter) (string, string, bool) {
	inventoryURL := globalConfig.GetInventoryURL()
	insecureSkipTLS := globalConfig.GetInventoryInsecureSkipTLS()

	if inventoryURL != "" {
		return inventoryURL, "available", insecureSkipTLS
	}

	return "not found", "not available", false
}

// GetMTVControllerInfo returns information about the MTV Operator
func GetMTVControllerInfo(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags) (string, string, string) {
	operatorInfo := client.GetMTVOperatorInfo(ctx, kubeConfigFlags)

	// Check for API/auth/network errors first
	if operatorInfo.Error != "" {
		return "unknown", "error: " + operatorInfo.Error, ""
	}

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
func GetVersionInfo(ctx context.Context, clientVersion string, kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig config.InventoryConfigGetter) Info {
	// Get MTV Operator information
	controllerVersion, controllerStatus, controllerNamespace := GetMTVControllerInfo(ctx, kubeConfigFlags)

	// Get inventory information from global config
	inventoryURL, inventoryStatus, inventoryInsecure := GetInventoryInfo(globalConfig)

	return Info{
		ClientVersion:     clientVersion,
		OperatorVersion:   controllerVersion,
		OperatorStatus:    controllerStatus,
		OperatorNamespace: controllerNamespace,
		InventoryURL:      inventoryURL,
		InventoryStatus:   inventoryStatus,
		InventoryInsecure: inventoryInsecure,
	}
}
