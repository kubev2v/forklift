package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yaacov/kubectl-mtv-mcp/pkg/mtvmcp"
)

// ComponentConfig defines how to find a specific component type
type ComponentConfig struct {
	Namespace string
	Selector  string
}

// Component presets based on cluster investigation
var ComponentSelectors = map[string]ComponentConfig{
	// Forklift/MTV components (openshift-mtv namespace)
	"controller": {
		Namespace: "openshift-mtv",
		Selector:  "control-plane=controller-manager",
	},
	"operator": {
		Namespace: "openshift-mtv",
		Selector:  "name=controller-manager",
	},
	"api": {
		Namespace: "openshift-mtv",
		Selector:  "service=forklift-api",
	},
	"validation": {
		Namespace: "openshift-mtv",
		Selector:  "service=forklift-validation",
	},
	"ui-plugin": {
		Namespace: "openshift-mtv",
		Selector:  "service=forklift-ui-plugin",
	},
	"cli-download": {
		Namespace: "openshift-mtv",
		Selector:  "service=forklift-cli-download",
	},
	"ova-proxy": {
		Namespace: "openshift-mtv",
		Selector:  "subapp=ova-proxy",
	},
	"forklift": {
		Namespace: "openshift-mtv",
		Selector:  "app=forklift",
	},

	// CDI components (openshift-cnv namespace)
	"cdi-operator": {
		Namespace: "openshift-cnv",
		Selector:  "cdi.kubevirt.io=cdi-operator",
	},
	"cdi-controller": {
		Namespace: "openshift-cnv",
		Selector:  "cdi.kubevirt.io=cdi-deployment",
	},
	"cdi-apiserver": {
		Namespace: "openshift-cnv",
		Selector:  "cdi.kubevirt.io=cdi-apiserver",
	},
	"cdi-uploadproxy": {
		Namespace: "openshift-cnv",
		Selector:  "cdi.kubevirt.io=cdi-uploadproxy",
	},
	"cdi": {
		Namespace: "openshift-cnv",
		Selector:  "app=containerized-data-importer",
	},

	// KubeVirt components (openshift-cnv namespace)
	"virt-operator": {
		Namespace: "openshift-cnv",
		Selector:  "kubevirt.io=virt-operator",
	},
	"virt-api": {
		Namespace: "openshift-cnv",
		Selector:  "kubevirt.io=virt-api",
	},
	"virt-controller": {
		Namespace: "openshift-cnv",
		Selector:  "kubevirt.io=virt-controller",
	},
	"virt-handler": {
		Namespace: "openshift-cnv",
		Selector:  "kubevirt.io=virt-handler",
	},
	"virt-exportproxy": {
		Namespace: "openshift-cnv",
		Selector:  "kubevirt.io=virt-exportproxy",
	},
}

// QueryPodsInput represents the input for QueryPods
type QueryPodsInput struct {
	Component      string `json:"component,omitempty" jsonschema:"Component type (controller, api, cdi-operator, virt-api, etc). See description for full list"`
	Namespace      string `json:"namespace,omitempty" jsonschema:"Kubernetes namespace to search in"`
	AllNamespaces  bool   `json:"all_namespaces,omitempty" jsonschema:"Search across all namespaces"`
	LabelSelector  string `json:"label_selector,omitempty" jsonschema:"Label selector (e.g., 'app=forklift' or 'kubevirt.io=virt-api')"`
	NamePattern    string `json:"name_pattern,omitempty" jsonschema:"Pod name pattern (regex, e.g., '^forklift-.*' or 'importer')"`
	ShowContainers bool   `json:"show_containers,omitempty" jsonschema:"Include container names and status in output"`
	DryRun         bool   `json:"dry_run,omitempty" jsonschema:"If true, shows commands instead of executing"`
}

// GetQueryPodsTool returns the tool definition
func GetQueryPodsTool() *mcp.Tool {
	return &mcp.Tool{
		Name: "QueryPods",
		Description: `Discover and query MTV/KubeVirt pods in the cluster.

    This tool helps discover pods without knowing exact names, using component types,
    label selectors, or name patterns. Based on actual cluster investigation.

    COMPONENT TYPES (preset configurations):
    
    Forklift/MTV (openshift-mtv namespace):
    - controller: Forklift controller (main orchestration) - containers: main, inventory
    - operator: Forklift operator
    - api: Forklift API server
    - validation: Forklift validation webhook
    - ui-plugin: Forklift UI plugin
    - cli-download: Forklift CLI download service
    - ova-proxy: OVA proxy for OVA migrations
    - forklift: All Forklift components
    
    CDI (openshift-cnv namespace):
    - cdi-operator: CDI operator
    - cdi-controller: CDI controller (main data import logic)
    - cdi-apiserver: CDI API server
    - cdi-uploadproxy: CDI upload proxy
    - cdi: All CDI components
    
    KubeVirt (openshift-cnv namespace):
    - virt-operator: KubeVirt operator
    - virt-api: KubeVirt API server
    - virt-controller: KubeVirt VM controller
    - virt-handler: KubeVirt node handlers (DaemonSet)
    - virt-exportproxy: KubeVirt export proxy

    SELECTION METHODS (use one):
    1. component: Use preset component type
    2. label_selector: Custom label selector
    3. name_pattern: Regex pattern to match pod names
    4. Just namespace: List all pods in namespace

    DISCOVERY STRATEGIES:
    
    Installation troubleshooting:
        QueryPods(component="operator")
        QueryPods(namespace="openshift-mtv")
    
    Operation monitoring:
        QueryPods(component="controller", show_containers=true)
        QueryPods(component="virt-handler")
    
    Migration debugging:
        QueryPods(namespace="demo", name_pattern="importer")
        QueryPods(label_selector="plan=<uuid>", namespace="demo")
    
    VM debugging:
        QueryPods(label_selector="vm.kubevirt.io/name=<vm-name>", namespace="<vm-ns>")
        QueryPods(label_selector="app=virt-launcher", namespace="<vm-ns>")

    Args:
        component: Component type from preset list (optional)
        namespace: Kubernetes namespace (optional, auto-detected for components)
        all_namespaces: Search across all namespaces
        label_selector: Custom label selector (optional)
        name_pattern: Pod name regex pattern (optional)
        show_containers: Include container info in output

    Returns:
        JSON with pod list and summary:
        {
            "pods": [
                {
                    "name": "pod-name",
                    "namespace": "namespace",
                    "phase": "Running",
                    "labels": {...},
                    "containers": [
                        {"name": "container1", "ready": true},
                        ...
                    ],
                    "age": "5d3h",
                    "node": "worker-1"
                }
            ],
            "summary": {
                "total": 10,
                "running": 9,
                "pending": 1,
                "by_component": {...}
            }
        }

    Examples:
        # Find controller pod
        QueryPods(component="controller", show_containers=true)
        
        # Find all Forklift components
        QueryPods(component="forklift")
        
        # Find importer pods in migration namespace
        QueryPods(namespace="demo", name_pattern="importer")
        
        # Find VM launcher pods
        QueryPods(label_selector="app=virt-launcher", namespace="production")
        
        # Find specific VM pod
        QueryPods(label_selector="vm.kubevirt.io/name=my-vm", namespace="production")`,
	}
}

func HandleQueryPods(ctx context.Context, req *mcp.CallToolRequest, input QueryPodsInput) (*mcp.CallToolResult, any, error) {
	// Enable dry run mode if requested
	if input.DryRun {
		ctx = mtvmcp.WithDryRun(ctx, true)
	}

	// Build kubectl command arguments
	args := []string{"get", "pods"}

	// Determine namespace
	namespace := input.Namespace
	if input.Component != "" {
		if config, ok := ComponentSelectors[input.Component]; ok {
			if namespace == "" {
				namespace = config.Namespace
			}
			if input.LabelSelector == "" {
				input.LabelSelector = config.Selector
			}
		} else {
			return nil, "", fmt.Errorf("unknown component type '%s'. Available: %v", input.Component, getComponentList())
		}
	}

	// Add namespace flags
	if input.AllNamespaces {
		args = append(args, "-A")
	} else if namespace != "" {
		args = append(args, "-n", namespace)
	}

	// Add label selector if provided
	if input.LabelSelector != "" {
		args = append(args, "-l", input.LabelSelector)
	}

	// Always get JSON output
	args = append(args, "-o", "json")

	// Execute kubectl command
	output, err := mtvmcp.RunKubectlCommand(ctx, args)
	if err != nil {
		return nil, "", fmt.Errorf("failed to query pods: %w", err)
	}

	stdout := mtvmcp.ExtractStdoutFromResponse(output)

	// Parse pod list
	var podList map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &podList); err != nil {
		return nil, "", fmt.Errorf("failed to parse pod list: %w", err)
	}

	// Filter by name pattern if provided
	if input.NamePattern != "" {
		podList, err = filterPodsByName(podList, input.NamePattern)
		if err != nil {
			return nil, "", fmt.Errorf("failed to filter by name pattern: %w", err)
		}
	}

	// Enhance pod data if show_containers is requested
	if input.ShowContainers {
		podList = enhancePodData(podList)
	}

	// Add summary
	result := map[string]interface{}{
		"pods":    podList["items"],
		"summary": generatePodSummary(podList),
	}

	return nil, result, nil
}

// filterPodsByName filters pods by name pattern
func filterPodsByName(podList map[string]interface{}, pattern string) (map[string]interface{}, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid name pattern: %w", err)
	}

	items, ok := podList["items"].([]interface{})
	if !ok {
		return podList, nil
	}

	filtered := make([]interface{}, 0)
	for _, item := range items {
		pod, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		metadata, ok := pod["metadata"].(map[string]interface{})
		if !ok {
			continue
		}

		name, ok := metadata["name"].(string)
		if ok && re.MatchString(name) {
			filtered = append(filtered, item)
		}
	}

	podList["items"] = filtered
	return podList, nil
}

// enhancePodData adds container information to pod data
func enhancePodData(podList map[string]interface{}) map[string]interface{} {
	items, ok := podList["items"].([]interface{})
	if !ok {
		return podList
	}

	for i, item := range items {
		pod, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		// Extract container info
		containers := make([]map[string]interface{}, 0)

		if spec, ok := pod["spec"].(map[string]interface{}); ok {
			if containerList, ok := spec["containers"].([]interface{}); ok {
				for _, c := range containerList {
					if container, ok := c.(map[string]interface{}); ok {
						containerInfo := map[string]interface{}{
							"name": container["name"],
						}

						// Get ready status from pod status
						if status, ok := pod["status"].(map[string]interface{}); ok {
							if containerStatuses, ok := status["containerStatuses"].([]interface{}); ok {
								for _, cs := range containerStatuses {
									if csMap, ok := cs.(map[string]interface{}); ok {
										if csMap["name"] == container["name"] {
											containerInfo["ready"] = csMap["ready"]
											containerInfo["restartCount"] = csMap["restartCount"]
											break
										}
									}
								}
							}
						}

						containers = append(containers, containerInfo)
					}
				}
			}
		}

		pod["_containers"] = containers
		items[i] = pod
	}

	podList["items"] = items
	return podList
}

// generatePodSummary creates a summary of pod statistics
func generatePodSummary(podList map[string]interface{}) map[string]interface{} {
	items, ok := podList["items"].([]interface{})
	if !ok {
		return map[string]interface{}{"total": 0}
	}

	summary := map[string]interface{}{
		"total": len(items),
	}

	phaseCount := make(map[string]int)
	componentCount := make(map[string]int)

	for _, item := range items {
		pod, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		// Count by phase
		if status, ok := pod["status"].(map[string]interface{}); ok {
			if phase, ok := status["phase"].(string); ok {
				phaseCount[strings.ToLower(phase)]++
			}
		}

		// Count by component (from labels)
		if metadata, ok := pod["metadata"].(map[string]interface{}); ok {
			if labels, ok := metadata["labels"].(map[string]interface{}); ok {
				// Try to identify component
				if service, ok := labels["service"].(string); ok {
					componentCount[service]++
				} else if comp, ok := labels["control-plane"].(string); ok {
					componentCount[comp]++
				} else if comp, ok := labels["kubevirt.io"].(string); ok {
					componentCount["kubevirt-"+comp]++
				} else if comp, ok := labels["cdi.kubevirt.io"].(string); ok {
					componentCount["cdi-"+comp]++
				}
			}
		}
	}

	// Add phase counts
	for phase, count := range phaseCount {
		summary[phase] = count
	}

	// Add component breakdown if any
	if len(componentCount) > 0 {
		summary["by_component"] = componentCount
	}

	return summary
}

// getComponentList returns list of available component types
func getComponentList() []string {
	components := make([]string, 0, len(ComponentSelectors))
	for k := range ComponentSelectors {
		components = append(components, k)
	}
	return components
}
