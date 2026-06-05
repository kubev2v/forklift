package health

import (
	"context"

	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

// CheckOperatorHealth checks the health of the MTV operator
func CheckOperatorHealth(ctx context.Context, configFlags *genericclioptions.ConfigFlags) OperatorHealth {
	health := OperatorHealth{
		Installed: false,
		Status:    "Not Installed",
	}

	// Get operator info using existing utility
	operatorInfo := client.GetMTVOperatorInfo(ctx, configFlags)

	// Check for API/auth/network errors first
	if operatorInfo.Error != "" {
		health.Status = "Unknown"
		health.Error = operatorInfo.Error
		return health
	}

	if !operatorInfo.Found {
		return health
	}

	health.Installed = true
	health.Version = operatorInfo.Version
	health.Namespace = operatorInfo.Namespace

	if health.Namespace == "" {
		health.Namespace = client.OpenShiftMTVNamespace
	}

	health.Status = "Installed"

	return health
}

// GetOperatorNamespace returns the MTV operator namespace
func GetOperatorNamespace(ctx context.Context, configFlags *genericclioptions.ConfigFlags) string {
	return client.GetMTVOperatorNamespace(ctx, configFlags)
}
