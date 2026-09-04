package powershell

import (
	"strings"
	"testing"
)

func TestBuildCommandInitiateShutdown(t *testing.T) {
	script := BuildCommand(InitiateShutdown, "test-vm")

	for _, want := range []string{
		"$vmName = 'test-vm'",
		"Get-CimInstance",
		"InitiateShutdown",
		"Msvm_ShutdownComponent",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("BuildCommand(InitiateShutdown, %q) missing %q\ngot: %s", "test-vm", want, script)
		}
	}
}

func TestBuildCommandInitiateShutdownQuoteEscaping(t *testing.T) {
	script := BuildCommand(InitiateShutdown, "vm'name")

	if !strings.Contains(script, "vm''name") {
		t.Errorf("BuildCommand(InitiateShutdown, %q) did not escape single quote, got: %s", "vm'name", script)
	}
	if strings.Contains(script, "$vmName = 'vm'name'") {
		t.Errorf("BuildCommand(InitiateShutdown, %q) produced unescaped quote, got: %s", "vm'name", script)
	}
}
