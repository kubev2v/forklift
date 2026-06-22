package storage_test

import (
	"testing"

	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/ontap"
	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/powerflex"
	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/powermax"
	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/primera3par"
	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/pure"
	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/storage"
)

func TestMatchesDevice_SameVendor_NonPF(t *testing.T) {
	tests := []struct {
		name       string
		provider   storage.ArrayIdentifier
		deviceName string
		want       bool
	}{
		{
			name:       "ONTAP matches ONTAP device",
			provider:   &ontap.NetappClonner{},
			deviceName: "naa.600a0980383139544924583130314c41",
			want:       true,
		},
		{
			name:       "ONTAP matches ONTAP device (uppercase)",
			provider:   &ontap.NetappClonner{},
			deviceName: "naa.600A0980383139544924583130314C41",
			want:       true,
		},
		{
			name:       "Pure matches Pure device",
			provider:   &pure.FlashArrayClonner{},
			deviceName: "naa.624a9370a7b9f7ecc01e40f70001181f",
			want:       true,
		},
		{
			name:       "3PAR matches 3PAR device",
			provider:   &primera3par.Primera3ParClonner{},
			deviceName: "naa.60002ac0000000000000182d00021f6b",
			want:       true,
		},
		{
			name:       "PowerMax matches PowerMax device",
			provider:   &powermax.PowermaxClonner{},
			deviceName: "naa.60000970000297700461533030333846",
			want:       true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.provider.MatchesDevice(tt.deviceName)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("MatchesDevice(%q) = %v, want %v", tt.deviceName, got, tt.want)
			}
		})
	}
}

func TestMatchesDevice_SamePF(t *testing.T) {
	// PowerFlex MatchesDevice is tested via the OtherToPF and LocalToPF tests
	// since PowerflexClonner has unexported fields (systemId) that can't be set
	// from outside the package. The positive case (same PF system) requires
	// a properly initialized client with a real systemId.
	// The cross-vendor negative cases below prove the prefix logic works.
}

func TestMatchesDevice_FromPF_ToOther(t *testing.T) {
	tests := []struct {
		name       string
		provider   storage.ArrayIdentifier
		deviceName string
		want       bool
	}{
		{
			name:       "ONTAP does not match PowerFlex EUI device",
			provider:   &ontap.NetappClonner{},
			deviceName: "eui.b4f2d5322f73780f5a5beec600000002",
			want:       false,
		},
		{
			name:       "Pure does not match PowerFlex EUI device",
			provider:   &pure.FlashArrayClonner{},
			deviceName: "eui.b4f2d5322f73780f5a5beec600000002",
			want:       false,
		},
		{
			name:       "3PAR does not match PowerFlex EUI device",
			provider:   &primera3par.Primera3ParClonner{},
			deviceName: "eui.b4f2d5322f73780f5a5beec600000002",
			want:       false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.provider.MatchesDevice(tt.deviceName)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("MatchesDevice(%q) = %v, want %v", tt.deviceName, got, tt.want)
			}
		})
	}
}

func TestMatchesDevice_OtherToPF(t *testing.T) {
	pf := &powerflex.PowerflexClonner{}

	tests := []struct {
		name       string
		deviceName string
		want       bool
	}{
		{
			name:       "PowerFlex does not match ONTAP NAA device",
			deviceName: "naa.600a0980383139544924583130314c41",
			want:       false,
		},
		{
			name:       "PowerFlex does not match Pure NAA device",
			deviceName: "naa.624a9370a7b9f7ecc01e40f70001181f",
			want:       false,
		},
		{
			name:       "PowerFlex does not match 3PAR NAA device",
			deviceName: "naa.60002ac0000000000000182d00021f6b",
			want:       false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := pf.MatchesDevice(tt.deviceName)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("MatchesDevice(%q) = %v, want %v", tt.deviceName, got, tt.want)
			}
		})
	}
}

func TestMatchesDevice_LocalToOther(t *testing.T) {
	localDevices := []string{
		"naa.55cd2e414d53564f",
		"t10.ATA_VBOX_HARDDISK_VB12345678",
		"mpx.vmhba0:C0:T0:L0",
	}

	providers := []struct {
		name     string
		provider storage.ArrayIdentifier
	}{
		{"ONTAP", &ontap.NetappClonner{}},
		{"Pure", &pure.FlashArrayClonner{}},
		{"3PAR", &primera3par.Primera3ParClonner{}},
		{"PowerMax", &powermax.PowermaxClonner{}},
	}

	for _, p := range providers {
		for _, dev := range localDevices {
			t.Run(p.name+"_vs_"+dev, func(t *testing.T) {
				got, err := p.provider.MatchesDevice(dev)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got {
					t.Errorf("%s.MatchesDevice(%q) = true, want false (local device)", p.name, dev)
				}
			})
		}
	}
}

func TestMatchesDevice_LocalToPF(t *testing.T) {
	pf := &powerflex.PowerflexClonner{}

	localDevices := []string{
		"naa.55cd2e414d53564f",
		"t10.ATA_VBOX_HARDDISK_VB12345678",
		"mpx.vmhba0:C0:T0:L0",
	}

	for _, dev := range localDevices {
		t.Run("PF_vs_"+dev, func(t *testing.T) {
			got, err := pf.MatchesDevice(dev)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got {
				t.Errorf("PowerFlex.MatchesDevice(%q) = true, want false (local device)", dev)
			}
		})
	}
}

func TestMatchesDevice_CrossVendor(t *testing.T) {
	tests := []struct {
		name       string
		provider   storage.ArrayIdentifier
		deviceName string
		want       bool
	}{
		{
			name:       "ONTAP does not match Pure device",
			provider:   &ontap.NetappClonner{},
			deviceName: "naa.624a9370a7b9f7ecc01e40f70001181f",
			want:       false,
		},
		{
			name:       "Pure does not match ONTAP device",
			provider:   &pure.FlashArrayClonner{},
			deviceName: "naa.600a0980383139544924583130314c41",
			want:       false,
		},
		{
			name:       "3PAR does not match ONTAP device",
			provider:   &primera3par.Primera3ParClonner{},
			deviceName: "naa.600a0980383139544924583130314c41",
			want:       false,
		},
		{
			name:       "ONTAP does not match 3PAR device",
			provider:   &ontap.NetappClonner{},
			deviceName: "naa.60002ac0000000000000182d00021f6b",
			want:       false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.provider.MatchesDevice(tt.deviceName)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("MatchesDevice(%q) = %v, want %v", tt.deviceName, got, tt.want)
			}
		})
	}
}
