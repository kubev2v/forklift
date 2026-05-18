package v1beta1

import (
	"testing"
)

func TestGetHyperVTransferMethod(t *testing.T) {
	tests := []struct {
		name     string
		settings map[string]string
		want     string
	}{
		{"explicit iscsi", map[string]string{HyperVTransferMethod: "iscsi"}, HyperVTransferMethodISCSI},
		{"uppercase ISCSI", map[string]string{HyperVTransferMethod: "ISCSI"}, HyperVTransferMethodISCSI},
		{"mixed case iScsi", map[string]string{HyperVTransferMethod: "iScsi"}, HyperVTransferMethodISCSI},
		{"explicit smb", map[string]string{HyperVTransferMethod: "smb"}, HyperVTransferMethodSMB},
		{"unknown value defaults to smb", map[string]string{HyperVTransferMethod: "nfs"}, HyperVTransferMethodSMB},
		{"key missing defaults to smb", map[string]string{}, HyperVTransferMethodSMB},
		{"nil settings defaults to smb", nil, HyperVTransferMethodSMB},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Provider{}
			p.Spec.Settings = tt.settings
			if got := p.GetHyperVTransferMethod(); got != tt.want {
				t.Errorf("GetHyperVTransferMethod() = %q, want %q", got, tt.want)
			}
		})
	}
}
