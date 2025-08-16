package tests

import (
	"testing"
)

// TestCopyOffloadThin validates the migration of a VM with a thin-provisioned disk.
// It creates a new TestFramework, which sets up a test VM on vSphere,
// migrates it to OpenShift using Forklift, and verifies that the copy-offload
// primitive was used.
// This test can be run in parallel with other copy-offload tests.
func TestCopyOffloadThin(t *testing.T) {
	t.Parallel()
	framework := NewTestFramework(t, "thin")
	framework.Run()
}
