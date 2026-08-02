//go:build validation

// Standalone validation test for PowerstoreClonner.TargetPorts() against a real PowerStore array.
//
// Usage:
//   POWERSTORE_HOST=<mgmt-url> POWERSTORE_USER=<user> POWERSTORE_PASS=<pass> \
//     go test -tags=validation -run TestPowerstoreClonner_TargetPorts -v ./internal/powerstore/

package powerstore

import (
	"os"
	"testing"
)

func TestPowerstoreClonner_TargetPorts(t *testing.T) {
	host := os.Getenv("POWERSTORE_HOST")
	user := os.Getenv("POWERSTORE_USER")
	pass := os.Getenv("POWERSTORE_PASS")
	if host == "" || user == "" || pass == "" {
		t.Skip("Set POWERSTORE_HOST, POWERSTORE_USER, POWERSTORE_PASS to run this test")
	}

	c, err := NewPowerstoreClonner(host, user, pass, true)
	if err != nil {
		t.Fatalf("failed to create PowerstoreClonner: %v", err)
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
