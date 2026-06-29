package builder

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	"github.com/kubev2v/forklift/pkg/provider/azure/controller/inventory"
)

func TestDetectOS(t *testing.T) {
	b := &Builder{log: logging.WithName("test")}

	tests := []struct {
		name   string
		vm     *inventory.VMDetails
		wantOS string
	}{
		{
			name:   "nil properties returns linux default",
			vm:     &inventory.VMDetails{},
			wantOS: DefaultLinux,
		},
		{
			name: "Windows OSType",
			vm: &inventory.VMDetails{
				VirtualMachine: armcompute.VirtualMachine{
					Properties: &armcompute.VirtualMachineProperties{
						StorageProfile: &armcompute.StorageProfile{
							OSDisk: &armcompute.OSDisk{
								OSType: to.Ptr(armcompute.OperatingSystemTypesWindows),
							},
						},
					},
				},
			},
			wantOS: DefaultWindows,
		},
		{
			name: "Linux OSType",
			vm: &inventory.VMDetails{
				VirtualMachine: armcompute.VirtualMachine{
					Properties: &armcompute.VirtualMachineProperties{
						StorageProfile: &armcompute.StorageProfile{
							OSDisk: &armcompute.OSDisk{
								OSType: to.Ptr(armcompute.OperatingSystemTypesLinux),
							},
						},
					},
				},
			},
			wantOS: DefaultLinux,
		},
		{
			name: "Ubuntu from image reference",
			vm: &inventory.VMDetails{
				VirtualMachine: armcompute.VirtualMachine{
					Properties: &armcompute.VirtualMachineProperties{
						StorageProfile: &armcompute.StorageProfile{
							OSDisk: &armcompute.OSDisk{},
							ImageReference: &armcompute.ImageReference{
								Offer: to.Ptr("UbuntuServer"),
							},
						},
					},
				},
			},
			wantOS: "ubuntu20.04",
		},
		{
			name: "RHEL from image reference",
			vm: &inventory.VMDetails{
				VirtualMachine: armcompute.VirtualMachine{
					Properties: &armcompute.VirtualMachineProperties{
						StorageProfile: &armcompute.StorageProfile{
							OSDisk: &armcompute.OSDisk{},
							ImageReference: &armcompute.ImageReference{
								Offer: to.Ptr("RHEL"),
							},
						},
					},
				},
			},
			wantOS: "rhel8.1",
		},
		{
			name: "Windows from image reference",
			vm: &inventory.VMDetails{
				VirtualMachine: armcompute.VirtualMachine{
					Properties: &armcompute.VirtualMachineProperties{
						StorageProfile: &armcompute.StorageProfile{
							OSDisk: &armcompute.OSDisk{},
							ImageReference: &armcompute.ImageReference{
								Offer: to.Ptr("WindowsServer"),
							},
						},
					},
				},
			},
			wantOS: DefaultWindows,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := b.detectOS(tt.vm)
			if got != tt.wantOS {
				t.Errorf("detectOS() = %q, want %q", got, tt.wantOS)
			}
		})
	}
}
