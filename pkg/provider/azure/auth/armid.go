package auth

import (
	"fmt"
	"strings"
)

const resourceGroupsSegment = "resourceGroups"

// ParseResourceGroup extracts the resource group name from an Azure ARM resource ID.
func ParseResourceGroup(armID string) (string, error) {
	parts := strings.Split(armID, "/")
	for i := 0; i < len(parts)-1; i++ {
		if parts[i] == resourceGroupsSegment && parts[i+1] != "" {
			return parts[i+1], nil
		}
	}
	return "", fmt.Errorf("resource group not found in ARM ID %q", armID)
}

// ParseResourceName returns the last path segment of an Azure ARM resource ID.
func ParseResourceName(armID string) string {
	parts := strings.Split(armID, "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

// IsARMResourceID reports whether id looks like a fully-qualified Azure ARM resource ID.
func IsARMResourceID(id string) bool {
	return strings.HasPrefix(id, "/subscriptions/")
}
