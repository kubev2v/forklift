package tests

import (
	"testing"
)

func TestCopyOffloadThin(t *testing.T) {
	framework := NewTestFramework(t, "thin")
	framework.Run()
}
