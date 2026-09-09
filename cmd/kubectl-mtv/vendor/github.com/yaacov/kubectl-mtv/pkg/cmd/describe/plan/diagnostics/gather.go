package diagnostics

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"

	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

// GatherDiagnostics collects diagnostics from all sources for the given plan and migration.
func GatherDiagnostics(ctx context.Context, configFlags *genericclioptions.ConfigFlags, dynClient dynamic.Interface, plan *unstructured.Unstructured, migration *unstructured.Unstructured, targetNS string, logLines, showLines int) (*DiagnosticsReport, error) {
	clientset, err := client.GetKubernetesClientset(configFlags)
	if err != nil {
		return nil, err
	}

	if logLines <= 0 {
		logLines = defaultLogTailLines
	}
	if showLines <= 0 {
		showLines = defaultShowLines
	}
	if logLines > MaxLogTailLines {
		logLines = MaxLogTailLines
	}
	if showLines > MaxShowLines {
		showLines = MaxShowLines
	}

	planUID := string(plan.GetUID())
	planName := plan.GetName()
	planNS := plan.GetNamespace()

	migrationUID := ""
	migrationName := ""
	if migration != nil {
		migrationUID = string(migration.GetUID())
		migrationName = migration.GetName()
	}

	localTarget := isLocalTarget(ctx, dynClient, plan)

	report := &DiagnosticsReport{
		PlanName:           planName,
		PlanUID:            planUID,
		MigrationName:      migrationName,
		MigrationUID:       migrationUID,
		TargetNS:           targetNS,
		RemoteTarget:       !localTarget,
		RequestedShowLines: showLines,
	}

	// Config context
	report.Config = CollectConfigContext(ctx, configFlags, dynClient, plan)

	// Controller logs (filtered by plan name/UID, with error analysis)
	report.ControllerLogs = CollectControllerLogs(ctx, configFlags, clientset, planName, planUID, logLines, showLines)

	// If no migration exists, return early with just config + controller logs
	if migration == nil {
		return report, nil
	}

	// Cutover time (relevant for warm migrations)
	cutover, _, _ := unstructured.NestedString(migration.Object, "spec", "cutover")
	report.CutoverTime = cutover

	// Extract per-VM diagnostics from migration status
	vms, _, _ := unstructured.NestedSlice(migration.Object, "status", "vms")
	for _, v := range vms {
		vmStatus, ok := v.(map[string]interface{})
		if !ok {
			continue
		}

		vmName, _, _ := unstructured.NestedString(vmStatus, "name")
		vmID, _, _ := unstructured.NestedString(vmStatus, "id")
		vmPhase, _, _ := unstructured.NestedString(vmStatus, "phase")

		vmDiag := VMDiagnostics{
			Name:  vmName,
			ID:    vmID,
			Phase: vmPhase,
		}

		// VM errors (always available from migration CR status)
		vmDiag.Error, vmDiag.Conditions, vmDiag.StepErrors = ExtractVMErrors(vmStatus)

		// Conversion CRs (in the plan namespace, always on local cluster)
		conversions := CollectConversions(ctx, dynClient, planNS, planName, vmID)
		if len(conversions) > 0 {
			vmDiag.Conversion = &conversions[0]
		}

		// Pods and events require access to the target cluster
		if localTarget {
			vmDiag.Pods = CollectPodDiagnostics(ctx, clientset, targetNS, planUID, migrationUID, vmID, logLines, showLines)

			// If conversion references a pod, collect its logs
			if vmDiag.Conversion != nil && vmDiag.Conversion.PodName != "" {
				convPod := CollectPodDiagnosticsByName(ctx, clientset, targetNS, vmDiag.Conversion.PodName, logLines, showLines)
				if convPod == nil && targetNS != planNS {
					convPod = CollectPodDiagnosticsByName(ctx, clientset, planNS, vmDiag.Conversion.PodName, logLines, showLines)
				}
				if convPod != nil {
					vmDiag.Pods = appendIfNew(vmDiag.Pods, *convPod)
				}
			}

			// Events (collect for all discovered pods + PVCs)
			podNames := make([]string, 0, len(vmDiag.Pods))
			for _, p := range vmDiag.Pods {
				podNames = append(podNames, p.Name)
			}
			vmDiag.Events = CollectEvents(ctx, clientset, targetNS, planUID, migrationUID, vmID, podNames)
		}

		report.VMs = append(report.VMs, vmDiag)
	}

	return report, nil
}

// isLocalTarget checks whether the plan's destination provider is the local cluster.
// The host provider uses an in-cluster connection and has no spec.url.
func isLocalTarget(ctx context.Context, dynClient dynamic.Interface, plan *unstructured.Unstructured) bool {
	destName, _, _ := unstructured.NestedString(plan.Object, "spec", "provider", "destination", "name")
	destNS, _, _ := unstructured.NestedString(plan.Object, "spec", "provider", "destination", "namespace")
	if destName == "" {
		return true
	}
	if destNS == "" {
		destNS = plan.GetNamespace()
	}

	provider, err := dynClient.Resource(client.ProvidersGVR).Namespace(destNS).Get(ctx, destName, metav1.GetOptions{})
	if err != nil {
		return true
	}

	provType, _, _ := unstructured.NestedString(provider.Object, "spec", "type")
	if provType != "openshift" {
		return true
	}

	url, _, _ := unstructured.NestedString(provider.Object, "spec", "url")
	return url == ""
}

func appendIfNew(pods []PodDiagnostics, pod PodDiagnostics) []PodDiagnostics {
	for _, p := range pods {
		if p.Name == pod.Name {
			return pods
		}
	}
	return append(pods, pod)
}
