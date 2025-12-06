package ova

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/ova"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	cnv "kubevirt.io/api/core/v1"
)

func TestMapFirmware(t *testing.T) {
	tests := []struct {
		name               string
		firmware           string
		secureBoot         bool
		expectedBootloader *cnv.Bootloader
		expectedSMM        *cnv.FeatureState
		migrationVMs       []*plan.VMStatus
	}{
		{
			name:       "UEFI with SecureBoot enabled",
			firmware:   "efi",
			secureBoot: true,
			expectedBootloader: &cnv.Bootloader{
				EFI: &cnv.EFI{
					SecureBoot: boolPtr(true),
				},
			},
			expectedSMM: &cnv.FeatureState{
				Enabled: boolPtr(true),
			},
		},
		{
			name:       "UEFI with SecureBoot disabled",
			firmware:   "efi",
			secureBoot: false,
			expectedBootloader: &cnv.Bootloader{
				EFI: &cnv.EFI{
					SecureBoot: boolPtr(false),
				},
			},
			expectedSMM: nil, // SMM should not be set when SecureBoot is disabled
		},
		{
			name:       "BIOS firmware ignores SecureBoot",
			firmware:   "bios",
			secureBoot: true, // This should be ignored for BIOS
			expectedBootloader: &cnv.Bootloader{
				BIOS: &cnv.BIOS{},
			},
			expectedSMM: nil, // SMM should not be set for BIOS
		},
		{
			name:       "Empty firmware defaults to UEFI without SecureBoot",
			firmware:   "",
			secureBoot: false,
			expectedBootloader: &cnv.Bootloader{
				EFI: &cnv.EFI{
					SecureBoot: boolPtr(false),
				},
			},
			expectedSMM:  nil,
			migrationVMs: nil,
		},
		{
			name:       "Empty firmware falls back to Migration.Status.VMs with SecureBoot",
			firmware:   "",
			secureBoot: true,
			expectedBootloader: &cnv.Bootloader{
				EFI: &cnv.EFI{
					SecureBoot: boolPtr(true),
				},
			},
			expectedSMM: &cnv.FeatureState{
				Enabled: boolPtr(true),
			},
			migrationVMs: []*plan.VMStatus{
				{
					VM: plan.VM{
						Ref: ref.Ref{
							ID: "test-vm-id",
						},
					},
					Firmware: "efi",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create migration VMs list (use test-specific or empty)
			migrationVMs := tt.migrationVMs
			if migrationVMs == nil {
				migrationVMs = []*plan.VMStatus{}
			}

			// Create a builder with minimal context
			builder := &Builder{
				Context: &plancontext.Context{
					Log: logging.WithName("test"),
					Migration: &api.Migration{
						Status: api.MigrationStatus{
							VMs: migrationVMs,
						},
					},
				},
			}

			// Create test VM
			vm := &model.VM{
				Firmware:   tt.firmware,
				SecureBoot: tt.secureBoot,
			}

			// Create test VM reference
			vmRef := ref.Ref{
				ID: "test-vm-id",
			}

			// Create empty VM spec
			vmSpec := &cnv.VirtualMachineSpec{
				Template: &cnv.VirtualMachineInstanceTemplateSpec{
					Spec: cnv.VirtualMachineInstanceSpec{
						Domain: cnv.DomainSpec{},
					},
				},
			}

			// Call mapFirmware
			builder.mapFirmware(vm, vmRef, vmSpec)

			// Verify bootloader
			if vmSpec.Template.Spec.Domain.Firmware == nil {
				t.Fatal("Firmware should be set but is nil")
			}

			bootloader := vmSpec.Template.Spec.Domain.Firmware.Bootloader
			if bootloader == nil {
				t.Fatal("Bootloader should be set but is nil")
			}

			// Check bootloader configuration
			if diff := cmp.Diff(tt.expectedBootloader, bootloader); diff != "" {
				t.Errorf("Bootloader mismatch (-want +got):\n%s", diff)
			}

			// Check SMM feature
			var actualSMM *cnv.FeatureState
			if vmSpec.Template.Spec.Domain.Features != nil {
				actualSMM = vmSpec.Template.Spec.Domain.Features.SMM
			}
			if diff := cmp.Diff(tt.expectedSMM, actualSMM); diff != "" {
				t.Errorf("SMM mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// Helper function
func boolPtr(b bool) *bool {
	return &b
}
