//go:build validation

// Standalone validation test for InfiniboxClonner.TargetPorts() against a real InfiniBox array.
//
// Usage:
//   INFINIBOX_HOST=<url> INFINIBOX_USER=<user> INFINIBOX_PASS=<pass> \
//     go test -tags=validation -run TestInfiniboxClonner_TargetPorts -v ./internal/infinibox/

package infinibox

import (
	"os"
	"testing"
)

func TestInfiniboxClonner_TargetPorts(t *testing.T) {
	host := os.Getenv("INFINIBOX_HOST")
	user := os.Getenv("INFINIBOX_USER")
	pass := os.Getenv("INFINIBOX_PASS")
	if host == "" || user == "" || pass == "" {
		t.Skip("Set INFINIBOX_HOST, INFINIBOX_USER, INFINIBOX_PASS to run this test")
	}

	c, err := NewInfiniboxClonner(host, user, pass, true)
	if err != nil {
		t.Fatalf("failed to create InfiniboxClonner: %v", err)
	}

	ports, err := c.TargetPorts()
	if err != nil {
		t.Fatalf("TargetPorts failed: %v", err)
	}
	if len(ports) == 0 {
		t.Fatalf("expected at least one target port, got none")
	}
	for _, p := range ports {
		t.Logf("target port: %s", p)
	}
}
