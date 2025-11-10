package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yaacov/kubectl-mtv-mcp/pkg/mtvmcp"
)

// findControllerPod finds the forklift-controller pod in the specified namespace
func findControllerPod(ctx context.Context, namespace string) (string, error) {
	output, err := mtvmcp.RunKubectlCommand(ctx, []string{"get", "pods", "-n", namespace, "-l", "app=forklift-controller", "-o", "jsonpath={.items[0].metadata.name}"})
	if err != nil {
		return "", fmt.Errorf("failed to find controller pod: %w", err)
	}

	podName := mtvmcp.ExtractStdoutFromResponse(output)
	podName = strings.TrimSpace(podName)
	if podName == "" {
		return "", fmt.Errorf("no controller pod found in namespace %s", namespace)
	}

	return podName, nil
}

// getControllerLogs retrieves logs from the controller pod
func getControllerLogs(ctx context.Context, container string, lines int, follow bool, namespace string) (*mcp.CallToolResult, any, error) {
	// Get MTV operator namespace if not provided
	if namespace == "" {
		versionOutput, err := mtvmcp.RunKubectlMTVCommand(ctx, []string{"version", "-o", "json"})
		if err != nil {
			return nil, "", fmt.Errorf("failed to get operator namespace: %w", err)
		}

		stdout := mtvmcp.ExtractStdoutFromResponse(versionOutput)
		var versionData map[string]interface{}
		if err := json.Unmarshal([]byte(stdout), &versionData); err != nil {
			return nil, "", fmt.Errorf("failed to parse version output: %w", err)
		}

		ns, ok := versionData["operatorNamespace"].(string)
		if !ok || ns == "" {
			return nil, "", fmt.Errorf("operatorNamespace not found in version output")
		}
		namespace = ns
	}

	// Find controller pod
	podName, err := findControllerPod(ctx, namespace)
	if err != nil {
		return nil, "", err
	}

	// Get pod information
	podInfoOutput, err := mtvmcp.RunKubectlCommand(ctx, []string{"get", "pod", "-n", namespace, podName, "-o", "json"})
	if err != nil {
		return nil, "", fmt.Errorf("failed to get pod info: %w", err)
	}

	podStdout := mtvmcp.ExtractStdoutFromResponse(podInfoOutput)
	var podInfo map[string]interface{}
	if err := json.Unmarshal([]byte(podStdout), &podInfo); err != nil {
		return nil, "", fmt.Errorf("failed to parse pod info: %w", err)
	}

	// Build kubectl logs command
	logsArgs := []string{"logs", "-n", namespace, podName}
	if container != "" {
		logsArgs = append(logsArgs, "-c", container)
	}
	if lines > 0 {
		logsArgs = append(logsArgs, "--tail", fmt.Sprintf("%d", lines))
	}
	if follow {
		logsArgs = append(logsArgs, "-f")
	}

	logsOutput, err := mtvmcp.RunKubectlCommand(ctx, logsArgs)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get logs: %w", err)
	}

	logsStdout := mtvmcp.ExtractStdoutFromResponse(logsOutput)

	result := map[string]interface{}{
		"pod":  podInfo,
		"logs": logsStdout,
	}

	return nil, result, nil
}

// getImporterLogs retrieves logs from an importer pod
func getImporterLogs(ctx context.Context, lines int, follow bool, namespace, planID, migrationID, vmID string) (*mcp.CallToolResult, any, error) {
	if planID == "" || migrationID == "" || vmID == "" {
		return nil, "", fmt.Errorf("plan_id, migration_id, and vm_id are required for importer pod logs")
	}

	if namespace == "" {
		return nil, "", fmt.Errorf("namespace is required for importer pod logs")
	}

	// Find PVCs with migration labels
	labelSelector := fmt.Sprintf("plan=%s,migration=%s,vmID=%s", planID, migrationID, vmID)
	pvcsOutput, err := mtvmcp.RunKubectlCommand(ctx, []string{"get", "pvc", "-n", namespace, "-l", labelSelector, "-o", "json"})
	if err != nil {
		return nil, "", fmt.Errorf("failed to get PVCs: %w", err)
	}

	pvcsStdout := mtvmcp.ExtractStdoutFromResponse(pvcsOutput)
	var pvcsData map[string]interface{}
	if err := json.Unmarshal([]byte(pvcsStdout), &pvcsData); err != nil {
		return nil, "", fmt.Errorf("failed to parse PVCs: %w", err)
	}

	pvcs, ok := pvcsData["items"].([]interface{})
	if !ok || len(pvcs) == 0 {
		return nil, "", fmt.Errorf("no PVCs found with labels plan=%s, migration=%s, vmID=%s", planID, migrationID, vmID)
	}

	// Find migration PVC UID
	var migrationPVCUID string
	for _, pvc := range pvcs {
		pvcMap, ok := pvc.(map[string]interface{})
		if !ok {
			continue
		}
		metadata, ok := pvcMap["metadata"].(map[string]interface{})
		if !ok {
			continue
		}
		uid, ok := metadata["uid"].(string)
		if ok && uid != "" {
			migrationPVCUID = uid
			break
		}
	}

	if migrationPVCUID == "" {
		return nil, "", fmt.Errorf("could not find migration PVC UID")
	}

	// Find prime PVC owned by migration PVC
	allPVCsOutput, err := mtvmcp.RunKubectlCommand(ctx, []string{"get", "pvc", "-n", namespace, "-o", "json"})
	if err != nil {
		return nil, "", fmt.Errorf("failed to get all PVCs: %w", err)
	}

	allPVCsStdout := mtvmcp.ExtractStdoutFromResponse(allPVCsOutput)
	var allPVCsData map[string]interface{}
	if err := json.Unmarshal([]byte(allPVCsStdout), &allPVCsData); err != nil {
		return nil, "", fmt.Errorf("failed to parse all PVCs: %w", err)
	}

	var importerPodName string
	allPVCs, ok := allPVCsData["items"].([]interface{})
	if !ok {
		return nil, "", fmt.Errorf("unexpected PVC list format: missing items[]")
	}
	for _, pvc := range allPVCs {
		pvcMap, ok := pvc.(map[string]interface{})
		if !ok {
			continue
		}
		metadata, ok := pvcMap["metadata"].(map[string]interface{})
		if !ok {
			continue
		}
		annotations, _ := metadata["annotations"].(map[string]interface{})

		// Check if this PVC is owned by our migration PVC
		owners, _ := metadata["ownerReferences"].([]interface{})
		for _, owner := range owners {
			ownerMap, ok := owner.(map[string]interface{})
			if !ok {
				continue
			}
			ownerUID, _ := ownerMap["uid"].(string)
			if ownerUID == migrationPVCUID {
				// This is a prime PVC, check for importer pod annotation
				if podName, ok := annotations["cdi.kubevirt.io/storage.import.importPodName"].(string); ok && podName != "" {
					importerPodName = podName
					break
				}
			}
		}
		if importerPodName != "" {
			break
		}
	}

	if importerPodName == "" {
		return nil, "", fmt.Errorf("could not find importer pod name in PVC annotations")
	}

	// Get pod information
	podInfoOutput, err := mtvmcp.RunKubectlCommand(ctx, []string{"get", "pod", "-n", namespace, importerPodName, "-o", "json"})
	if err != nil {
		return nil, "", fmt.Errorf("failed to get pod info: %w", err)
	}

	podStdout := mtvmcp.ExtractStdoutFromResponse(podInfoOutput)
	var podInfo map[string]interface{}
	if err := json.Unmarshal([]byte(podStdout), &podInfo); err != nil {
		return nil, "", fmt.Errorf("failed to parse pod info: %w", err)
	}

	// Build kubectl logs command
	logsArgs := []string{"logs", "-n", namespace, importerPodName}
	if lines > 0 {
		logsArgs = append(logsArgs, "--tail", fmt.Sprintf("%d", lines))
	}
	if follow {
		logsArgs = append(logsArgs, "-f")
	}

	logsOutput, err := mtvmcp.RunKubectlCommand(ctx, logsArgs)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get logs: %w", err)
	}

	logsStdout := mtvmcp.ExtractStdoutFromResponse(logsOutput)

	result := map[string]interface{}{
		"pod":  podInfo,
		"logs": logsStdout,
	}

	return nil, result, nil
}

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
