package tests

import (
	"testing"
)

func TestCopyOffloadEagerZeroed(t *testing.T) {
	framework := NewTestFramework(t, "eagerzeroedthick")
	framework.Run()
}
