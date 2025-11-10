package hook

import (
	"context"
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/output"
)

// colorizeYAML adds syntax highlighting to YAML content
func colorizeYAML(yamlContent string) string {
	lines := strings.Split(yamlContent, "\n")
	var colorizedLines []string

	// Compile regex patterns once outside the loop for better performance
	listItemRegex := regexp.MustCompile(`^\s*-\s+`)
	listItemReplaceRegex := regexp.MustCompile(`^(\s*-\s+)(.*)`)

	for _, line := range lines {
		// Color comments
		if strings.TrimSpace(line) != "" && strings.HasPrefix(strings.TrimSpace(line), "#") {
			colorizedLines = append(colorizedLines, output.ColorizedString(line, output.CyanColor))
			continue
		}

		// Color key-value pairs
		if strings.Contains(line, ":") {
			// Split on first colon to separate key from value
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				key := parts[0]
				value := parts[1]

				// Color the key in blue and value in yellow
				coloredKey := output.Blue(key)
				coloredValue := value
				if strings.TrimSpace(value) != "" {
					coloredValue = output.Yellow(value)
				}
				colorizedLines = append(colorizedLines, coloredKey+":"+coloredValue)
				continue
			}
		}

		// Color list items (lines starting with -)
		if listItemRegex.MatchString(line) {
			colored := listItemReplaceRegex.ReplaceAllStringFunc(line, func(match string) string {
				submatches := listItemReplaceRegex.FindStringSubmatch(match)
				if len(submatches) >= 3 {
					return output.Green(submatches[1]) + submatches[2]
				}
				return match
			})
			colorizedLines = append(colorizedLines, colored)
			continue
		}

		// Default: no coloring
		colorizedLines = append(colorizedLines, line)
	}

	return strings.Join(colorizedLines, "\n")
}

// Describe describes a migration hook
func Describe(configFlags *genericclioptions.ConfigFlags, name, namespace string, useUTC bool) error {
	c, err := client.GetDynamicClient(configFlags)
	if err != nil {
		return fmt.Errorf("failed to get client: %v", err)
	}

	// Get the hook
	hook, err := c.Resource(client.HooksGVR).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get hook: %v", err)
	}

	// Print the hook details
	fmt.Printf("\n%s", output.ColorizedSeparator(105, output.YellowColor))
	fmt.Printf("\n%s\n", output.Cyan("MIGRATION HOOK"))

	// Basic Information
	fmt.Printf("%s %s\n", output.Bold("Name:"), output.Yellow(hook.GetName()))
	fmt.Printf("%s %s\n", output.Bold("Namespace:"), output.Yellow(hook.GetNamespace()))
	fmt.Printf("%s %s\n", output.Bold("Created:"), output.Yellow(output.FormatTimestamp(hook.GetCreationTimestamp().Time, useUTC)))

	// Hook Spec Information
	if image, found, _ := unstructured.NestedString(hook.Object, "spec", "image"); found {
		fmt.Printf("%s %s\n", output.Bold("Image:"), output.Yellow(image))
	}

	if serviceAccount, found, _ := unstructured.NestedString(hook.Object, "spec", "serviceAccount"); found && serviceAccount != "" {
		fmt.Printf("%s %s\n", output.Bold("Service Account:"), output.Yellow(serviceAccount))
	} else {
		fmt.Printf("%s %s\n", output.Bold("Service Account:"), output.Yellow("(default)"))
	}

	if deadline, found, _ := unstructured.NestedInt64(hook.Object, "spec", "deadline"); found && deadline > 0 {
		fmt.Printf("%s %d seconds\n", output.Bold("Deadline:"), deadline)
	} else {
		fmt.Printf("%s %s\n", output.Bold("Deadline:"), output.Yellow("(unlimited)"))
	}

	// Playbook Information
	playbook, playbookFound, _ := unstructured.NestedString(hook.Object, "spec", "playbook")
	if playbookFound && playbook != "" {
		fmt.Printf("%s %s\n", output.Bold("Playbook:"), output.Green("Yes"))

		// Decode and display playbook content
		if decoded, err := base64.StdEncoding.DecodeString(playbook); err == nil {
			fmt.Printf("\n%s\n", output.Cyan("PLAYBOOK CONTENT"))
			fmt.Printf("%s\n%s\n%s\n",
				output.ColorizedString("```yaml", output.BoldYellow),
				colorizeYAML(string(decoded)),
				output.ColorizedString("```", output.BoldYellow))
		} else {
			fmt.Printf("%s %s\n", output.Bold("Playbook Decoding:"), output.Red("Failed - invalid base64"))
		}
	} else {
		fmt.Printf("%s %s\n", output.Bold("Playbook:"), output.Yellow("No"))
	}

	// Status Information
	if status, found, _ := unstructured.NestedMap(hook.Object, "status"); found && status != nil {
		fmt.Printf("\n%s\n", output.Cyan("STATUS"))

		// Conditions
		if conditions, found, _ := unstructured.NestedSlice(hook.Object, "status", "conditions"); found {
			fmt.Printf("%s\n", output.Bold("Conditions:"))
			for _, condition := range conditions {
				if condMap, ok := condition.(map[string]interface{}); ok {
					condType, _ := condMap["type"].(string)
					condStatus, _ := condMap["status"].(string)
					reason, _ := condMap["reason"].(string)
					message, _ := condMap["message"].(string)
					lastTransitionTime, _ := condMap["lastTransitionTime"].(string)

					fmt.Printf("  %s: %s", output.Bold(condType), output.ColorizeStatus(condStatus))
					if reason != "" {
						fmt.Printf(" (%s)", reason)
					}
					fmt.Println()

					if message != "" {
						fmt.Printf("    %s\n", message)
					}
					if lastTransitionTime != "" {
						fmt.Printf("    Last Transition: %s\n", lastTransitionTime)
					}
				}
			}
		}

		// Other status fields
		if observedGeneration, found, _ := unstructured.NestedInt64(hook.Object, "status", "observedGeneration"); found {
			fmt.Printf("%s %d\n", output.Bold("Observed Generation:"), observedGeneration)
		}
	}

	// Owner References
	if len(hook.GetOwnerReferences()) > 0 {
		fmt.Printf("\n%s\n", output.Cyan("OWNERSHIP"))
		for _, owner := range hook.GetOwnerReferences() {
			fmt.Printf("%s %s/%s", output.Bold("Owner:"), owner.Kind, owner.Name)
			if owner.Controller != nil && *owner.Controller {
				fmt.Printf(" %s", output.Green("(controller)"))
			}
			fmt.Println()
		}
	}

	// Annotations
	if annotations := hook.GetAnnotations(); len(annotations) > 0 {
		fmt.Printf("\n%s\n", output.Cyan("ANNOTATIONS"))
		for key, value := range annotations {
			fmt.Printf("%s: %s\n", output.Bold(key), value)
		}
	}

	// Labels
	if labels := hook.GetLabels(); len(labels) > 0 {
		fmt.Printf("\n%s\n", output.Cyan("LABELS"))
		for key, value := range labels {
			fmt.Printf("%s: %s\n", output.Bold(key), value)
		}
	}

	// Usage Information
	fmt.Printf("\n%s\n", output.Cyan("USAGE"))
	fmt.Printf("This hook can be referenced in migration plans per VM using:\n")

	// Create colored YAML example
	yamlExample := fmt.Sprintf(`spec:
  vms:
    - id: <vm_id>
      hooks:
        - hook:
            namespace: %s
            name: %s
          step: PreHook  # or PostHook`, hook.GetNamespace(), hook.GetName())

	fmt.Printf("%s\n%s\n%s\n",
		output.ColorizedString("```yaml", output.BoldYellow),
		colorizeYAML(yamlExample),
		output.ColorizedString("```", output.BoldYellow))
	fmt.Printf("\n%s: For a PreHook to run on a VM, the VM must be started and available via SSH.\n", output.Bold("Note"))

	fmt.Println() // Add a newline at the end
	return nil
}
