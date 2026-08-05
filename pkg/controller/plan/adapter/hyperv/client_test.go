package hyperv

import (
	"testing"

	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/hyperv"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/hyperv"
)

func TestParseStateString(t *testing.T) {
	c := &Client{}
	tests := []struct {
		name     string
		raw      string
		expected planapi.VMPowerState
	}{
		{"off lowercase", "Off", planapi.VMPowerStateOff},
		{"off with whitespace", "  Off\r\n", planapi.VMPowerStateOff},
		{"running", "Running", planapi.VMPowerStateOn},
		{"running with whitespace", "\n Running \n", planapi.VMPowerStateOn},
		{"unknown state", "Paused", planapi.VMPowerStateUnknown},
		{"empty string", "", planapi.VMPowerStateUnknown},
		{"shutoff variant", "TurnedOff", planapi.VMPowerStateOff},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := c.parseStateString(tc.raw)
			if got != tc.expected {
				t.Errorf("parseStateString(%q) = %q, want %q", tc.raw, got, tc.expected)
			}
		})
	}
}

func TestPowerStateFromInventory(t *testing.T) {
	c := &Client{}
	tests := []struct {
		name     string
		state    string
		expected planapi.VMPowerState
	}{
		{"on", model.PowerStateOn, planapi.VMPowerStateOn},
		{"off", model.PowerStateOff, planapi.VMPowerStateOff},
		{"paused", model.PowerStatePaused, planapi.VMPowerStateUnknown},
		{"empty", "", planapi.VMPowerStateUnknown},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			vm := &hyperv.VM{PowerState: tc.state}
			got := c.powerStateFromInventory(vm)
			if got != tc.expected {
				t.Errorf("powerStateFromInventory(state=%q) = %q, want %q",
					tc.state, got, tc.expected)
			}
		})
	}
}
