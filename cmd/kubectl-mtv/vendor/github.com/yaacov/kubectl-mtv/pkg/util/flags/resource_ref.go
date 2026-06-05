package flags

import (
	"fmt"
	"strings"
)

// ParseResourceRef parses a resource reference in the form "name" or
// "namespace/name". When no slash is present, defaultNamespace is used.
// Both namespace and name must be non-empty after trimming whitespace.
func ParseResourceRef(input, defaultNamespace string) (namespace, name string, err error) {
	if strings.Contains(input, "/") {
		parts := strings.SplitN(input, "/", 2)
		namespace = strings.TrimSpace(parts[0])
		name = strings.TrimSpace(parts[1])
		if namespace == "" {
			return "", "", fmt.Errorf("invalid resource reference %q: namespace must not be empty", input)
		}
		if name == "" {
			return "", "", fmt.Errorf("invalid resource reference %q: name must not be empty", input)
		}
		return namespace, name, nil
	}

	name = strings.TrimSpace(input)
	if name == "" {
		return "", "", fmt.Errorf("invalid resource reference: name must not be empty")
	}
	return defaultNamespace, name, nil
}
