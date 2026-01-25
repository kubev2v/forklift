package validator

import (
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	"github.com/kubev2v/forklift/pkg/provider/ec2/inventory/model"
	"github.com/kubev2v/forklift/pkg/provider/ec2/inventory/web"
	"github.com/kubev2v/forklift/pkg/provider/testutil"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

func TestValidator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "EC2 controller validator")
}

// FakeInventory implements the base.Client interface for testing.
type FakeInventory struct {
	VMs     map[string]*web.VM
	Volumes map[string]*web.Volume
	Error   error
}

func NewFakeInventory() *FakeInventory {
	return &FakeInventory{
		VMs:     make(map[string]*web.VM),
		Volumes: make(map[string]*web.Volume),
	}
}

// Compile-time check
var _ base.Client = (*FakeInventory)(nil)

// Finder implements base.Client
func (f *FakeInventory) Finder() base.Finder { return nil }

// Get implements base.Client
func (f *FakeInventory) Get(resource interface{}, id string) error {
	return errors.New("not implemented")
}

// List implements base.Client
func (f *FakeInventory) List(list interface{}, param ...base.Param) error {
	return errors.New("not implemented")
}

// Watch implements base.Client
func (f *FakeInventory) Watch(resource interface{}, h base.EventHandler) (*base.Watch, error) {
	return nil, errors.New("not implemented")
}

// Find implements base.Client
func (f *FakeInventory) Find(resource interface{}, r ref.Ref) error {
	if f.Error != nil {
		return f.Error
	}

	switch res := resource.(type) {
	case *web.VM:
		id := r.ID
		if id == "" {
			id = r.Name
		}
		if vm, ok := f.VMs[id]; ok {
			*res = *vm
			return nil
		}
		return errors.New("VM not found")
	case *web.Volume:
		id := r.ID
		if id == "" {
			id = r.Name
		}
		if vol, ok := f.Volumes[id]; ok {
			*res = *vol
			return nil
		}
		return errors.New("Volume not found")
	}
	return errors.New("unknown resource type")
}

// VM implements base.Client
func (f *FakeInventory) VM(r *ref.Ref) (interface{}, error) {
	return nil, errors.New("not implemented")
}

// Workload implements base.Client
func (f *FakeInventory) Workload(r *ref.Ref) (interface{}, error) {
	return nil, errors.New("not implemented")
}

// Network implements base.Client
func (f *FakeInventory) Network(r *ref.Ref) (interface{}, error) {
	return nil, errors.New("not implemented")
}

// Storage implements base.Client
func (f *FakeInventory) Storage(r *ref.Ref) (interface{}, error) {
	return nil, errors.New("not implemented")
}

// Host implements base.Client
func (f *FakeInventory) Host(r *ref.Ref) (interface{}, error) {
	return nil, errors.New("not implemented")
}

var _ = Describe("EC2 Controller Validator", func() {
	var (
		validator  *Validator
		fakeInv    *FakeInventory
		networkMap *api.NetworkMap
		storageMap *api.StorageMap
	)

	BeforeEach(func() {
		fakeInv = NewFakeInventory()

		// Create network map
		networkMap = &api.NetworkMap{
			Spec: api.NetworkMapSpec{
				Map: []api.NetworkPair{
					{
						Source: ref.Ref{ID: "subnet-123"},
						Destination: api.DestinationNetwork{
							Type: "pod",
						},
					},
				},
			},
		}

		// Create storage map
		storageMap = &api.StorageMap{
			Spec: api.StorageMapSpec{
				Map: []api.StoragePair{
					{
						Source:      ref.Ref{Name: "gp2"},
						Destination: api.DestinationStorage{StorageClass: "standard"},
					},
					{
						Source:      ref.Ref{Name: "gp3"},
						Destination: api.DestinationStorage{StorageClass: "premium-rwo"},
					},
				},
			},
		}

		// Create plan context using testutil
		ctx := testutil.NewContextBuilder().
			WithNetworkMap(networkMap).
			WithStorageMap(storageMap).
			Build()

		// Set the fake inventory
		ctx.Source.Inventory = fakeInv

		// Create validator
		validator = New(ctx)
	})

	Describe("MigrationType", func() {
		It("should return true for empty migration type", func() {
			validator.Context.Plan.Spec.Type = ""
			Expect(validator.MigrationType()).To(BeTrue())
		})

		It("should return true for cold migration", func() {
			validator.Context.Plan.Spec.Type = api.MigrationCold
			Expect(validator.MigrationType()).To(BeTrue())
		})

		table.DescribeTable("should return false for unsupported migration types",
			func(migrationType api.MigrationType) {
				validator.Context.Plan.Spec.Type = migrationType
				Expect(validator.MigrationType()).To(BeFalse())
			},
			table.Entry("warm migration", api.MigrationWarm),
		)
	})

	Describe("validateStorage", func() {
		It("should pass when VM has EBS volumes", func() {
			instanceDetails := createInstanceWithEBS("i-123", "vol-111", "vol-222")
			fakeInv.VMs["i-123"] = &web.VM{
				Resource: web.Resource{ID: "i-123"},
				Object:   instanceDetails,
			}

			vmRef := ref.Ref{ID: "i-123"}
			ok, err := validator.validateStorage(vmRef)

			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
		})

		It("should fail when VM has no block devices", func() {
			instanceDetails := &model.InstanceDetails{
				ID:                  "i-123",
				BlockDeviceMappings: []model.InstanceBlockDeviceMapping{},
			}
			fakeInv.VMs["i-123"] = &web.VM{
				Resource: web.Resource{ID: "i-123"},
				Object:   instanceDetails,
			}

			vmRef := ref.Ref{ID: "i-123"}
			ok, err := validator.validateStorage(vmRef)

			Expect(err).To(HaveOccurred())
			Expect(ok).To(BeFalse())
			Expect(err.Error()).To(ContainSubstring("no block devices"))
		})

		It("should fail when VM has only instance store volumes", func() {
			instanceDetails := &model.InstanceDetails{
				ID: "i-123",
				BlockDeviceMappings: []model.InstanceBlockDeviceMapping{
					{
						DeviceName:  aws.String("/dev/sdb"),
						VirtualName: aws.String("ephemeral0"),
					},
				},
			}
			fakeInv.VMs["i-123"] = &web.VM{
				Resource: web.Resource{ID: "i-123"},
				Object:   instanceDetails,
			}

			vmRef := ref.Ref{ID: "i-123"}
			ok, err := validator.validateStorage(vmRef)

			Expect(err).To(HaveOccurred())
			Expect(ok).To(BeFalse())
			Expect(err.Error()).To(ContainSubstring("only instance store"))
		})

		It("should pass when VM has mixed EBS and instance store", func() {
			instanceDetails := &model.InstanceDetails{
				ID: "i-123",
				BlockDeviceMappings: []model.InstanceBlockDeviceMapping{
					{
						DeviceName: aws.String("/dev/sda1"),
						Ebs:        &model.EbsInstanceBlockDevice{VolumeId: aws.String("vol-111")},
					},
					{
						DeviceName:  aws.String("/dev/sdb"),
						VirtualName: aws.String("ephemeral0"),
					},
				},
			}
			fakeInv.VMs["i-123"] = &web.VM{
				Resource: web.Resource{ID: "i-123"},
				Object:   instanceDetails,
			}

			vmRef := ref.Ref{ID: "i-123"}
			ok, err := validator.validateStorage(vmRef)

			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
		})

		It("should fail when VM not found in inventory", func() {
			vmRef := ref.Ref{ID: "i-nonexistent"}
			ok, err := validator.validateStorage(vmRef)

			Expect(err).To(HaveOccurred())
			Expect(ok).To(BeFalse())
		})
	})

	Describe("NetworksMapped", func() {
		It("should pass when all network interfaces are mapped", func() {
			instanceDetails := &model.InstanceDetails{
				ID: "i-123",
				NetworkInterfaces: []model.InstanceNetworkInterface{
					{SubnetId: aws.String("subnet-123")},
				},
			}
			fakeInv.VMs["i-123"] = &web.VM{
				Resource: web.Resource{ID: "i-123"},
				Object:   instanceDetails,
			}

			vmRef := ref.Ref{ID: "i-123"}
			ok, err := validator.NetworksMapped(vmRef)

			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
		})

		It("should fail when network interface is not mapped", func() {
			instanceDetails := &model.InstanceDetails{
				ID: "i-123",
				NetworkInterfaces: []model.InstanceNetworkInterface{
					{SubnetId: aws.String("subnet-unmapped")},
				},
			}
			fakeInv.VMs["i-123"] = &web.VM{
				Resource: web.Resource{ID: "i-123"},
				Object:   instanceDetails,
			}

			vmRef := ref.Ref{ID: "i-123"}
			ok, err := validator.NetworksMapped(vmRef)

			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeFalse())
		})

		It("should pass when VM has no network interfaces", func() {
			instanceDetails := &model.InstanceDetails{
				ID:                "i-123",
				NetworkInterfaces: []model.InstanceNetworkInterface{},
			}
			fakeInv.VMs["i-123"] = &web.VM{
				Resource: web.Resource{ID: "i-123"},
				Object:   instanceDetails,
			}

			vmRef := ref.Ref{ID: "i-123"}
			ok, err := validator.NetworksMapped(vmRef)

			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
		})

		It("should skip interfaces with nil SubnetId", func() {
			instanceDetails := &model.InstanceDetails{
				ID: "i-123",
				NetworkInterfaces: []model.InstanceNetworkInterface{
					{SubnetId: nil},
					{SubnetId: aws.String("subnet-123")},
				},
			}
			fakeInv.VMs["i-123"] = &web.VM{
				Resource: web.Resource{ID: "i-123"},
				Object:   instanceDetails,
			}

			vmRef := ref.Ref{ID: "i-123"}
			ok, err := validator.NetworksMapped(vmRef)

			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
		})
	})

	Describe("StorageMapped", func() {
		It("should pass when all volume types are mapped", func() {
			instanceDetails := createInstanceWithEBS("i-123", "vol-111")
			fakeInv.VMs["i-123"] = &web.VM{
				Resource: web.Resource{ID: "i-123"},
				Object:   instanceDetails,
			}

			// Add volume to inventory with gp3 type (which is mapped)
			volumeDetails := &model.VolumeDetails{}
			volumeDetails.VolumeType = ec2types.VolumeTypeGp3
			fakeInv.Volumes["vol-111"] = &web.Volume{
				Resource: web.Resource{ID: "vol-111"},
				Object:   volumeDetails,
			}

			vmRef := ref.Ref{ID: "i-123"}
			ok, err := validator.StorageMapped(vmRef)

			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
		})

		It("should fail when volume type is not mapped", func() {
			instanceDetails := createInstanceWithEBS("i-123", "vol-111")
			fakeInv.VMs["i-123"] = &web.VM{
				Resource: web.Resource{ID: "i-123"},
				Object:   instanceDetails,
			}

			// Add volume to inventory with io2 type (which is NOT mapped)
			volumeDetails := &model.VolumeDetails{}
			volumeDetails.VolumeType = ec2types.VolumeTypeIo2
			fakeInv.Volumes["vol-111"] = &web.Volume{
				Resource: web.Resource{ID: "vol-111"},
				Object:   volumeDetails,
			}

			vmRef := ref.Ref{ID: "i-123"}
			ok, err := validator.StorageMapped(vmRef)

			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeFalse())
		})

		It("should pass when VM has no block devices", func() {
			instanceDetails := &model.InstanceDetails{
				ID:                  "i-123",
				BlockDeviceMappings: []model.InstanceBlockDeviceMapping{},
			}
			fakeInv.VMs["i-123"] = &web.VM{
				Resource: web.Resource{ID: "i-123"},
				Object:   instanceDetails,
			}

			vmRef := ref.Ref{ID: "i-123"}
			ok, err := validator.StorageMapped(vmRef)

			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
		})

		It("should skip instance store volumes", func() {
			instanceDetails := &model.InstanceDetails{
				ID: "i-123",
				BlockDeviceMappings: []model.InstanceBlockDeviceMapping{
					{
						DeviceName:  aws.String("/dev/sdb"),
						VirtualName: aws.String("ephemeral0"),
					},
				},
			}
			fakeInv.VMs["i-123"] = &web.VM{
				Resource: web.Resource{ID: "i-123"},
				Object:   instanceDetails,
			}

			vmRef := ref.Ref{ID: "i-123"}
			ok, err := validator.StorageMapped(vmRef)

			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
		})
	})

	Describe("UnSupportedDisks", func() {
		It("should return instance store devices", func() {
			instanceDetails := &model.InstanceDetails{
				ID: "i-123",
				BlockDeviceMappings: []model.InstanceBlockDeviceMapping{
					{
						DeviceName: aws.String("/dev/sda1"),
						Ebs:        &model.EbsInstanceBlockDevice{VolumeId: aws.String("vol-111")},
					},
					{
						DeviceName:  aws.String("/dev/sdb"),
						VirtualName: aws.String("ephemeral0"),
					},
					{
						DeviceName:  aws.String("/dev/sdc"),
						VirtualName: aws.String("ephemeral1"),
					},
				},
			}
			fakeInv.VMs["i-123"] = &web.VM{
				Resource: web.Resource{ID: "i-123"},
				Object:   instanceDetails,
			}

			vmRef := ref.Ref{ID: "i-123"}
			unsupported, err := validator.UnSupportedDisks(vmRef)

			Expect(err).NotTo(HaveOccurred())
			Expect(unsupported).To(HaveLen(2))
			Expect(unsupported[0]).To(ContainSubstring("/dev/sdb"))
			Expect(unsupported[0]).To(ContainSubstring("ephemeral0"))
		})

		It("should return empty list when no instance store", func() {
			instanceDetails := createInstanceWithEBS("i-123", "vol-111")
			fakeInv.VMs["i-123"] = &web.VM{
				Resource: web.Resource{ID: "i-123"},
				Object:   instanceDetails,
			}

			vmRef := ref.Ref{ID: "i-123"}
			unsupported, err := validator.UnSupportedDisks(vmRef)

			Expect(err).NotTo(HaveOccurred())
			Expect(unsupported).To(BeEmpty())
		})

		It("should return nil when VM has no block devices", func() {
			instanceDetails := &model.InstanceDetails{
				ID:                  "i-123",
				BlockDeviceMappings: []model.InstanceBlockDeviceMapping{},
			}
			fakeInv.VMs["i-123"] = &web.VM{
				Resource: web.Resource{ID: "i-123"},
				Object:   instanceDetails,
			}

			vmRef := ref.Ref{ID: "i-123"}
			unsupported, err := validator.UnSupportedDisks(vmRef)

			Expect(err).NotTo(HaveOccurred())
			Expect(unsupported).To(BeNil())
		})
	})

	Describe("Validate (integration)", func() {
		It("should pass when all validations pass", func() {
			// Setup VM with EBS volume and mapped network
			instanceDetails := &model.InstanceDetails{
				ID: "i-123",
				BlockDeviceMappings: []model.InstanceBlockDeviceMapping{
					{
						DeviceName: aws.String("/dev/sda1"),
						Ebs:        &model.EbsInstanceBlockDevice{VolumeId: aws.String("vol-111")},
					},
				},
				NetworkInterfaces: []model.InstanceNetworkInterface{
					{SubnetId: aws.String("subnet-123")},
				},
			}
			fakeInv.VMs["i-123"] = &web.VM{
				Resource: web.Resource{ID: "i-123"},
				Object:   instanceDetails,
			}

			// Add volume with mapped type
			volumeDetails := &model.VolumeDetails{}
			volumeDetails.VolumeType = ec2types.VolumeTypeGp3
			fakeInv.Volumes["vol-111"] = &web.Volume{
				Resource: web.Resource{ID: "vol-111"},
				Object:   volumeDetails,
			}

			vmRef := ref.Ref{ID: "i-123"}
			ok, err := validator.Validate(vmRef)

			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
		})

		It("should fail when storage validation fails", func() {
			instanceDetails := &model.InstanceDetails{
				ID:                  "i-123",
				BlockDeviceMappings: []model.InstanceBlockDeviceMapping{},
			}
			fakeInv.VMs["i-123"] = &web.VM{
				Resource: web.Resource{ID: "i-123"},
				Object:   instanceDetails,
			}

			vmRef := ref.Ref{ID: "i-123"}
			ok, err := validator.Validate(vmRef)

			Expect(err).To(HaveOccurred())
			Expect(ok).To(BeFalse())
		})

		It("should fail when network mapping fails", func() {
			instanceDetails := &model.InstanceDetails{
				ID: "i-123",
				BlockDeviceMappings: []model.InstanceBlockDeviceMapping{
					{
						DeviceName: aws.String("/dev/sda1"),
						Ebs:        &model.EbsInstanceBlockDevice{VolumeId: aws.String("vol-111")},
					},
				},
				NetworkInterfaces: []model.InstanceNetworkInterface{
					{SubnetId: aws.String("subnet-unmapped")},
				},
			}
			fakeInv.VMs["i-123"] = &web.VM{
				Resource: web.Resource{ID: "i-123"},
				Object:   instanceDetails,
			}

			vmRef := ref.Ref{ID: "i-123"}
			ok, err := validator.Validate(vmRef)

			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeFalse())
		})

		It("should fail when storage mapping fails", func() {
			instanceDetails := &model.InstanceDetails{
				ID: "i-123",
				BlockDeviceMappings: []model.InstanceBlockDeviceMapping{
					{
						DeviceName: aws.String("/dev/sda1"),
						Ebs:        &model.EbsInstanceBlockDevice{VolumeId: aws.String("vol-111")},
					},
				},
				NetworkInterfaces: []model.InstanceNetworkInterface{
					{SubnetId: aws.String("subnet-123")},
				},
			}
			fakeInv.VMs["i-123"] = &web.VM{
				Resource: web.Resource{ID: "i-123"},
				Object:   instanceDetails,
			}

			// Add volume with unmapped type
			volumeDetails := &model.VolumeDetails{}
			volumeDetails.VolumeType = ec2types.VolumeTypeIo2
			fakeInv.Volumes["vol-111"] = &web.Volume{
				Resource: web.Resource{ID: "vol-111"},
				Object:   volumeDetails,
			}

			vmRef := ref.Ref{ID: "i-123"}
			ok, err := validator.Validate(vmRef)

			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeFalse())
		})
	})
})

// Helper function to create instance with EBS volumes
func createInstanceWithEBS(instanceID string, volumeIDs ...string) *model.InstanceDetails {
	instance := &model.InstanceDetails{
		ID:                  instanceID,
		BlockDeviceMappings: []model.InstanceBlockDeviceMapping{},
	}
	instance.InstanceId = aws.String(instanceID)

	for i, volID := range volumeIDs {
		device := model.InstanceBlockDeviceMapping{
			DeviceName: aws.String("/dev/sd" + string(rune('a'+i))),
			Ebs:        &model.EbsInstanceBlockDevice{VolumeId: aws.String(volID)},
		}
		instance.BlockDeviceMappings = append(instance.BlockDeviceMappings, device)
	}

	return instance
}
