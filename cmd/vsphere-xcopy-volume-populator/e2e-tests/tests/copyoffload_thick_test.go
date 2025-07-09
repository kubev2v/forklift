package tests

import (
	"testing"
)

// TestCopyOffloadThick validates the migration of a VM with a thick-provisioned disk.
// It creates a new TestFramework, which sets up a test VM on vSphere,
// migrates it to OpenShift using Forklift, and verifies that the copy-offload
// primitive was used.
// This test can be run in parallel with other copy-offload tests.
func TestCopyOffloadThick(t *testing.T) {
	t.Parallel()
	framework := NewTestFramework(t, "thick")
	framework.Run()
}
