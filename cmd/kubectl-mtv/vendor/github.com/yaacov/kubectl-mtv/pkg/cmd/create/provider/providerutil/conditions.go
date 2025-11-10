package providerutil

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ProviderConditionStatuses holds the formatted status values for different provider condition types
type ProviderConditionStatuses struct {
	ConnectionStatus string
	ValidationStatus string
	InventoryStatus  string
	ReadyStatus      string
}

// ExtractProviderConditionStatuses extracts and formats provider condition statuses from an unstructured object
func ExtractProviderConditionStatuses(obj map[string]interface{}) ProviderConditionStatuses {
	// Default condition values
	statuses := ProviderConditionStatuses{
		ConnectionStatus: "Unknown",
		ValidationStatus: "Unknown",
		InventoryStatus:  "Unknown",
		ReadyStatus:      "Unknown",
	}

	// Extract conditions
	conditions, exists, _ := unstructured.NestedSlice(obj, "status", "conditions")
	if !exists {
		return statuses
	}

	for _, c := range conditions {
		condition, ok := c.(map[string]interface{})
		if !ok {
			continue
		}

		condType, _, _ := unstructured.NestedString(condition, "type")
		condStatus, _, _ := unstructured.NestedString(condition, "status")

		// Convert status to a simpler display
		displayStatus := "Unknown"
		if condStatus == "True" {
			displayStatus = "True"
		} else if condStatus == "False" {
			displayStatus = "False"
		}

		// Map each condition to its corresponding status
		switch condType {
		case "ConnectionTestSucceeded":
			statuses.ConnectionStatus = displayStatus
		case "Validated":
			statuses.ValidationStatus = displayStatus
		case "InventoryCreated":
			statuses.InventoryStatus = displayStatus
		case "Ready":
			statuses.ReadyStatus = displayStatus
		}
	}

	return statuses
}
