package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yaacov/kubectl-mtv-mcp/pkg/mtvmcp"
)

// getMigrationPVCs retrieves PVCs for a migration
func getMigrationPVCs(ctx context.Context, migrationID, planID, vmID, namespace string, allNamespaces bool) (*mcp.CallToolResult, any, error) {
	args := []string{"get", "pvc"}

	if allNamespaces {
		args = append(args, "-A")
	} else if namespace != "" {
		args = append(args, "-n", namespace)
	}

	// Build label selector
	var labels []string
	if migrationID != "" {
		labels = append(labels, fmt.Sprintf("migration=%s", migrationID))
	}
	if planID != "" {
		labels = append(labels, fmt.Sprintf("plan=%s", planID))
	}
	if vmID != "" {
		labels = append(labels, fmt.Sprintf("vmID=%s", vmID))
	}

	if len(labels) > 0 {
		args = append(args, "-l", strings.Join(labels, ","))
	}

	args = append(args, "-o", "json")

	output, err := mtvmcp.RunKubectlCommand(ctx, args)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get PVCs: %w", err)
	}

	stdout := mtvmcp.ExtractStdoutFromResponse(output)

	// Parse and enhance with describe output
	var pvcsData map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &pvcsData); err != nil {
		return nil, "", fmt.Errorf("failed to parse PVCs: %w", err)
	}

	// Add describe output for each PVC
	if items, ok := pvcsData["items"].([]interface{}); ok {
		for i, item := range items {
			if pvcMap, ok := item.(map[string]interface{}); ok {
				if metadata, ok := pvcMap["metadata"].(map[string]interface{}); ok {
					pvcName, ok := metadata["name"].(string)
					if !ok {
						continue
					}
					pvcNs := namespace
					if allNamespaces {
						if ns, ok := metadata["namespace"].(string); ok {
							pvcNs = ns
						}
					}

					descArgs := []string{"describe", "pvc"}
					if pvcNs != "" {
						descArgs = append(descArgs, "-n", pvcNs)
					}
					descArgs = append(descArgs, pvcName)
					describeOutput, _ := mtvmcp.RunKubectlCommand(ctx, descArgs)
					describeStdout := mtvmcp.ExtractStdoutFromResponse(describeOutput)
					pvcMap["describe"] = describeStdout
					items[i] = pvcMap
				}
			}
		}
		pvcsData["items"] = items
	}

	return nil, pvcsData, nil
}

// getMigrationDataVolumes retrieves DataVolumes for a migration
func getMigrationDataVolumes(ctx context.Context, migrationID, planID, vmID, namespace string, allNamespaces bool) (*mcp.CallToolResult, any, error) {
	args := []string{"get", "datavolume"}

	if allNamespaces {
		args = append(args, "-A")
	} else if namespace != "" {
		args = append(args, "-n", namespace)
	}

	// Build label selector
	var labels []string
	if migrationID != "" {
		labels = append(labels, fmt.Sprintf("migration=%s", migrationID))
	}
	if planID != "" {
		labels = append(labels, fmt.Sprintf("plan=%s", planID))
	}
	if vmID != "" {
		labels = append(labels, fmt.Sprintf("vmID=%s", vmID))
	}

	if len(labels) > 0 {
		args = append(args, "-l", strings.Join(labels, ","))
	}

	args = append(args, "-o", "json")

	output, err := mtvmcp.RunKubectlCommand(ctx, args)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get DataVolumes: %w", err)
	}

	stdout := mtvmcp.ExtractStdoutFromResponse(output)

	// Parse and enhance with describe output
	var dvsData map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &dvsData); err != nil {
		return nil, "", fmt.Errorf("failed to parse DataVolumes: %w", err)
	}

	// Add describe output for each DataVolume
	if items, ok := dvsData["items"].([]interface{}); ok {
		for i, item := range items {
			if dvMap, ok := item.(map[string]interface{}); ok {
				if metadata, ok := dvMap["metadata"].(map[string]interface{}); ok {
					dvName, ok := metadata["name"].(string)
					if !ok {
						continue
					}
					dvNs := namespace
					if allNamespaces {
						if ns, ok := metadata["namespace"].(string); ok {
							dvNs = ns
						}
					}

					descArgs := []string{"describe", "datavolume"}
					if dvNs != "" {
						descArgs = append(descArgs, "-n", dvNs)
					}
					descArgs = append(descArgs, dvName)
					describeOutput, _ := mtvmcp.RunKubectlCommand(ctx, descArgs)
					describeStdout := mtvmcp.ExtractStdoutFromResponse(describeOutput)
					dvMap["describe"] = describeStdout
					items[i] = dvMap
				}
			}
		}
		dvsData["items"] = items
	}

	return nil, dvsData, nil
}
