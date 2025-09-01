//go:build e2e
// +build e2e

package tests

import (
	"testing"

	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/e2e-tests/helpers"
)

// TestCopyOffloadThin validates the migration of a VM with a thin-provisioned disk.
// It creates a new TestFramework, which sets up a test VM on vSphere,
// migrates it to OpenShift using Forklift, and verifies that the copy-offload
// primitive was used.
func TestCopyOffloadThin(t *testing.T) {
	// TBD: will be implemented in the future
	// t.Parallel()
	framework := NewTestFramework(t, helpers.DiskThin)
	framework.Run()
}
