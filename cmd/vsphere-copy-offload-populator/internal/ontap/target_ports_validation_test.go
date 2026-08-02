//go:build validation

// Standalone validation test for NetappClonner.TargetPorts() against a real ONTAP array.
//
// Usage:
//   ONTAP_HOST=<mgmt-lif> ONTAP_USER=<user> ONTAP_PASS=<pass> ONTAP_SVM=<svm> \
//     go test -tags=validation -run TestNetappClonner_TargetPorts -v ./internal/ontap/

package ontap

import (
	"os"
	"testing"
)

func TestNetappClonner_TargetPorts(t *testing.T) {
	host := os.Getenv("ONTAP_HOST")
	user := os.Getenv("ONTAP_USER")
	pass := os.Getenv("ONTAP_PASS")
	svm := os.Getenv("ONTAP_SVM")
	if host == "" || user == "" || pass == "" || svm == "" {
		t.Skip("Set ONTAP_HOST, ONTAP_USER, ONTAP_PASS, ONTAP_SVM to run this test")
	}

	os.Setenv("ONTAP_SVM", svm)
	c, err := NewNetappClonner(host, user, pass, true)
	if err != nil {
		t.Fatalf("failed to create NetappClonner: %v", err)
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
