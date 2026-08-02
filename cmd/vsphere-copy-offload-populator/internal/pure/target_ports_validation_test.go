//go:build validation

// Standalone validation test for FlashArrayClonner.TargetPorts() against a real Pure array.
//
// Usage:
//   PURE_HOST=<mgmt-hostname> PURE_USER=<user> PURE_PASS=<pass> \
//     go test -tags=validation -run TestFlashArrayClonner_TargetPorts -v ./internal/pure/

package pure

import (
	"os"
	"testing"
)

func TestFlashArrayClonner_TargetPorts(t *testing.T) {
	host := os.Getenv("PURE_HOST")
	user := os.Getenv("PURE_USER")
	pass := os.Getenv("PURE_PASS")
	if host == "" || user == "" || pass == "" {
		t.Skip("Set PURE_HOST, PURE_USER, PURE_PASS to run this test")
	}

	c, err := NewFlashArrayClonner(host, user, pass, "", true, "mtv", 30)
	if err != nil {
		t.Fatalf("failed to create FlashArrayClonner: %v", err)
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
