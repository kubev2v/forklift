//go:build e2e
// +build e2e

package tests

import (
	"testing"

	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/e2e-tests/helpers"
)

// TestCopyOffloadThick validates the migration of a VM with a thick-provisioned disk.
// It creates a new TestFramework, which sets up a test VM on vSphere,
// migrates it to OpenShift using Forklift, and verifies that the copy-offload
// primitive was used.
func TestCopyOffloadThick(t *testing.T) {
	// TBD: will be implemented in the future
	// t.Parallel()
	framework := NewTestFramework(t, helpers.DiskThick)
	framework.Run()
}
