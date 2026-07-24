package diagnostics

import (
	"context"
	"fmt"
	"sort"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// eventEntryWithTime is an internal type used for chronological sorting before
// the raw timestamp is converted to a display-friendly Age string.
type eventEntryWithTime struct {
	entry     EventEntry
	timestamp time.Time
}

// CollectEvents gathers Kubernetes events for all resources (pods, PVCs) associated
// with a migration. It uses label-based discovery to find PVC names, then queries
// events by involvedObject.name for both pods and PVCs.
func CollectEvents(ctx context.Context, clientset *kubernetes.Clientset, namespace, planUID, migrationUID, vmID string, podNames []string) []EventEntry {
	// Discover PVC names via labels
	pvcNames := discoverPVCNames(ctx, clientset, namespace, planUID, migrationUID, vmID)

	// Combine all object names for event queries
	allNames := make([]string, 0, len(podNames)+len(pvcNames))
	allNames = append(allNames, podNames...)
	allNames = append(allNames, pvcNames...)

	seen := make(map[string]bool)
	var collected []eventEntryWithTime

	for _, name := range allNames {
		events, err := clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
			FieldSelector: fmt.Sprintf("involvedObject.name=%s", name),
		})
		if err != nil || len(events.Items) == 0 {
			continue
		}

		for _, ev := range events.Items {
			if !isRelevantEvent(ev.Type, ev.Reason) {
				continue
			}

			key := fmt.Sprintf("%s/%s/%s", ev.Reason, ev.InvolvedObject.Name, ev.Message)
			if seen[key] {
				continue
			}
			seen[key] = true

			collected = append(collected, eventEntryWithTime{
				entry: EventEntry{
					Type:    ev.Type,
					Reason:  ev.Reason,
					Object:  fmt.Sprintf("%s/%s", ev.InvolvedObject.Kind, ev.InvolvedObject.Name),
					Message: truncate(ev.Message, 120),
					Age:     formatAge(ev.LastTimestamp.Time),
				},
				timestamp: ev.LastTimestamp.Time,
			})
		}
	}

	sort.Slice(collected, func(i, j int) bool {
		return collected[i].timestamp.Before(collected[j].timestamp)
	})

	entries := make([]EventEntry, len(collected))
	for i, c := range collected {
		entries[i] = c.entry
	}
	return entries
}

func discoverPVCNames(ctx context.Context, clientset *kubernetes.Clientset, namespace, planUID, migrationUID, vmID string) []string {
	selector := fmt.Sprintf("plan=%s,migration=%s", planUID, migrationUID)
	if vmID != "" {
		selector += fmt.Sprintf(",vmID=%s", vmID)
	}

	pvcs, err := clientset.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil || len(pvcs.Items) == 0 {
		return nil
	}

	names := make([]string, 0, len(pvcs.Items))
	for _, pvc := range pvcs.Items {
		names = append(names, pvc.Name)
	}
	return names
}

func isRelevantEvent(eventType, reason string) bool {
	if eventType == "Warning" {
		return true
	}
	relevantNormalReasons := map[string]bool{
		"Started":                true,
		"FailedScheduling":       true,
		"Evicted":                true,
		"ProvisioningFailed":     true,
		"ProvisioningSucceeded":  true,
		"SuccessfulAttachVolume": true,
	}
	return relevantNormalReasons[reason]
}

func formatAge(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
