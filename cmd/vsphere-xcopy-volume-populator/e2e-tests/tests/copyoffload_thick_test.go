package tests

import (
	"testing"
)

func TestCopyOffloadThick(t *testing.T) {
	framework := NewTestFramework(t, "thick")
	framework.Run()
}
