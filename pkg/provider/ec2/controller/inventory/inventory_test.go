package inventory

import (
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"github.com/kubev2v/forklift/pkg/provider/ec2/inventory/model"
	"github.com/kubev2v/forklift/pkg/provider/ec2/inventory/web"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

func TestInventory(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "EC2 controller inventory")
}

// FakeInventory implements the Inventory interface for testing.
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

var _ = Describe("EC2 Controller Inventory", func() {
	Describe("GetAWSInstance", func() {
		var fakeInv *FakeInventory

		BeforeEach(func() {
			fakeInv = NewFakeInventory()
		})

		It("should return instance details when VM exists", func() {
			instanceDetails := &model.InstanceDetails{
				ID:   "i-123",
				Name: "test-vm",
			}
			instanceDetails.InstanceId = aws.String("i-123")
			fakeInv.VMs["i-123"] = &web.VM{
				Resource: web.Resource{ID: "i-123", Name: "test-vm"},
				Object:   instanceDetails,
			}

			vmRef := ref.Ref{ID: "i-123"}
			result, err := GetAWSInstance(fakeInv, vmRef)

			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())
			Expect(result.ID).To(Equal("i-123"))
			Expect(result.Name).To(Equal("test-vm"))
		})

		It("should return error when VM not found", func() {
			vmRef := ref.Ref{ID: "i-nonexistent"}
			result, err := GetAWSInstance(fakeInv, vmRef)

			Expect(err).To(HaveOccurred())
			Expect(result).To(BeNil())
		})

		It("should return ErrNoAWSInstanceObject when Object is nil", func() {
			fakeInv.VMs["i-123"] = &web.VM{
				Resource: web.Resource{ID: "i-123", Name: "test-vm"},
				Object:   nil,
			}

			vmRef := ref.Ref{ID: "i-123"}
			result, err := GetAWSInstance(fakeInv, vmRef)

			Expect(err).To(Equal(ErrNoAWSInstanceObject))
			Expect(result).To(BeNil())
		})

		It("should return error when inventory fails", func() {
			fakeInv.Error = errors.New("inventory error")

			vmRef := ref.Ref{ID: "i-123"}
			result, err := GetAWSInstance(fakeInv, vmRef)

			Expect(err).To(HaveOccurred())
			Expect(result).To(BeNil())
		})
	})

	Describe("GetBlockDevices", func() {
		It("should return block devices when present", func() {
			instance := &model.InstanceDetails{
				BlockDeviceMappings: []model.InstanceBlockDeviceMapping{
					{DeviceName: aws.String("/dev/sda1")},
					{DeviceName: aws.String("/dev/sdb")},
				},
			}

			devices, found := GetBlockDevices(instance)

			Expect(found).To(BeTrue())
			Expect(devices).To(HaveLen(2))
		})

		It("should return false when no block devices", func() {
			instance := &model.InstanceDetails{
				BlockDeviceMappings: []model.InstanceBlockDeviceMapping{},
			}

			devices, found := GetBlockDevices(instance)

			Expect(found).To(BeFalse())
			Expect(devices).To(BeEmpty())
		})

		It("should return false when block devices is nil", func() {
			instance := &model.InstanceDetails{}

			devices, found := GetBlockDevices(instance)

			Expect(found).To(BeFalse())
			Expect(devices).To(BeNil())
		})
	})

	Describe("GetNetworkInterfaces", func() {
		It("should return network interfaces when present", func() {
			instance := &model.InstanceDetails{
				NetworkInterfaces: []model.InstanceNetworkInterface{
					{SubnetId: aws.String("subnet-123")},
					{SubnetId: aws.String("subnet-456")},
				},
			}

			interfaces, found := GetNetworkInterfaces(instance)

			Expect(found).To(BeTrue())
			Expect(interfaces).To(HaveLen(2))
		})

		It("should return false when no network interfaces", func() {
			instance := &model.InstanceDetails{
				NetworkInterfaces: []model.InstanceNetworkInterface{},
			}

			interfaces, found := GetNetworkInterfaces(instance)

			Expect(found).To(BeFalse())
			Expect(interfaces).To(BeEmpty())
		})
	})

	Describe("GetInstanceName", func() {
		table.DescribeTable("should return correct name",
			func(instance *model.InstanceDetails, expected string) {
				Expect(GetInstanceName(instance)).To(Equal(expected))
			},
			table.Entry("returns Name when set",
				&model.InstanceDetails{Name: "my-instance"},
				"my-instance"),
			table.Entry("returns InstanceId when Name is empty",
				func() *model.InstanceDetails {
					d := &model.InstanceDetails{}
					d.InstanceId = aws.String("i-123")
					return d
				}(),
				"i-123"),
			table.Entry("returns empty when both are empty",
				&model.InstanceDetails{},
				""),
		)
	})

	Describe("GetInstanceID", func() {
		It("should return instance ID when set", func() {
			instance := &model.InstanceDetails{}
			instance.InstanceId = aws.String("i-123")

			Expect(GetInstanceID(instance)).To(Equal("i-123"))
		})

		It("should return empty string when nil", func() {
			instance := &model.InstanceDetails{}

			Expect(GetInstanceID(instance)).To(Equal(""))
		})
	})

	Describe("IsInstanceStore", func() {
		table.DescribeTable("should correctly identify instance store",
			func(device model.InstanceBlockDeviceMapping, expected bool) {
				Expect(IsInstanceStore(device)).To(Equal(expected))
			},
			table.Entry("true when VirtualName is set",
				model.InstanceBlockDeviceMapping{VirtualName: aws.String("ephemeral0")},
				true),
			table.Entry("false when VirtualName is nil",
				model.InstanceBlockDeviceMapping{VirtualName: nil},
				false),
			table.Entry("false when VirtualName is empty string",
				model.InstanceBlockDeviceMapping{VirtualName: aws.String("")},
				false),
			table.Entry("false for EBS volume",
				model.InstanceBlockDeviceMapping{
					Ebs: &model.EbsInstanceBlockDevice{VolumeId: aws.String("vol-123")},
				},
				false),
		)
	})

	Describe("IsEBSVolume", func() {
		table.DescribeTable("should correctly identify EBS volume",
			func(device model.InstanceBlockDeviceMapping, expected bool) {
				Expect(IsEBSVolume(device)).To(Equal(expected))
			},
			table.Entry("true when Ebs with VolumeId is set",
				model.InstanceBlockDeviceMapping{
					Ebs: &model.EbsInstanceBlockDevice{VolumeId: aws.String("vol-123")},
				},
				true),
			table.Entry("false when Ebs is nil",
				model.InstanceBlockDeviceMapping{},
				false),
			table.Entry("false when VolumeId is nil",
				model.InstanceBlockDeviceMapping{
					Ebs: &model.EbsInstanceBlockDevice{VolumeId: nil},
				},
				false),
			table.Entry("false for instance store",
				model.InstanceBlockDeviceMapping{
					VirtualName: aws.String("ephemeral0"),
					Ebs:         &model.EbsInstanceBlockDevice{VolumeId: aws.String("vol-123")},
				},
				false),
		)
	})

	Describe("ExtractEBSVolumeID", func() {
		It("should return volume ID for EBS device", func() {
			device := model.InstanceBlockDeviceMapping{
				Ebs: &model.EbsInstanceBlockDevice{VolumeId: aws.String("vol-123")},
			}

			Expect(ExtractEBSVolumeID(device)).To(Equal("vol-123"))
		})

		It("should return empty string for non-EBS device", func() {
			device := model.InstanceBlockDeviceMapping{
				VirtualName: aws.String("ephemeral0"),
			}

			Expect(ExtractEBSVolumeID(device)).To(Equal(""))
		})

		It("should return empty string for empty device", func() {
			device := model.InstanceBlockDeviceMapping{}

			Expect(ExtractEBSVolumeID(device)).To(Equal(""))
		})
	})

	Describe("GetDeviceName", func() {
		It("should return device name when set", func() {
			device := model.InstanceBlockDeviceMapping{
				DeviceName: aws.String("/dev/sda1"),
			}

			Expect(GetDeviceName(device)).To(Equal("/dev/sda1"))
		})

		It("should return empty string when nil", func() {
			device := model.InstanceBlockDeviceMapping{}

			Expect(GetDeviceName(device)).To(Equal(""))
		})
	})

	Describe("GetVirtualName", func() {
		It("should return virtual name when set", func() {
			device := model.InstanceBlockDeviceMapping{
				VirtualName: aws.String("ephemeral0"),
			}

			Expect(GetVirtualName(device)).To(Equal("ephemeral0"))
		})

		It("should return empty string when nil", func() {
			device := model.InstanceBlockDeviceMapping{}

			Expect(GetVirtualName(device)).To(Equal(""))
		})
	})

	Describe("GetVolume", func() {
		var fakeInv *FakeInventory

		BeforeEach(func() {
			fakeInv = NewFakeInventory()
		})

		It("should return volume when found", func() {
			volumeDetails := &model.VolumeDetails{
				ID:   "vol-123",
				Name: "test-volume",
			}
			volumeDetails.VolumeType = ec2types.VolumeTypeGp3
			fakeInv.Volumes["vol-123"] = &web.Volume{
				Resource: web.Resource{ID: "vol-123", Name: "test-volume"},
				Object:   volumeDetails,
			}

			result, err := GetVolume(fakeInv, "vol-123")

			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())
			Expect(result.ID).To(Equal("vol-123"))
		})

		It("should return error when volume not found", func() {
			result, err := GetVolume(fakeInv, "vol-nonexistent")

			Expect(err).To(HaveOccurred())
			Expect(result).To(BeNil())
		})
	})

	Describe("GetVolumeType", func() {
		var fakeInv *FakeInventory

		BeforeEach(func() {
			fakeInv = NewFakeInventory()
		})

		It("should return volume type when found", func() {
			volumeDetails := &model.VolumeDetails{}
			volumeDetails.VolumeType = ec2types.VolumeTypeGp3
			fakeInv.Volumes["vol-123"] = &web.Volume{
				Resource: web.Resource{ID: "vol-123"},
				Object:   volumeDetails,
			}

			result := GetVolumeType(fakeInv, "vol-123")

			Expect(result).To(Equal("gp3"))
		})

		It("should return empty string for empty volumeID", func() {
			result := GetVolumeType(fakeInv, "")

			Expect(result).To(Equal(""))
		})

		It("should return empty string when volume not found", func() {
			result := GetVolumeType(fakeInv, "vol-nonexistent")

			Expect(result).To(Equal(""))
		})

		It("should return empty string when Object is nil", func() {
			fakeInv.Volumes["vol-123"] = &web.Volume{
				Resource: web.Resource{ID: "vol-123"},
				Object:   nil,
			}

			result := GetVolumeType(fakeInv, "vol-123")

			Expect(result).To(Equal(""))
		})
	})

	Describe("GetVolumeSize", func() {
		var fakeInv *FakeInventory

		BeforeEach(func() {
			fakeInv = NewFakeInventory()
		})

		It("should return volume size when found", func() {
			volumeDetails := &model.VolumeDetails{}
			volumeDetails.Size = aws.Int32(100)
			fakeInv.Volumes["vol-123"] = &web.Volume{
				Resource: web.Resource{ID: "vol-123"},
				Object:   volumeDetails,
			}

			result := GetVolumeSize(fakeInv, "vol-123")

			Expect(result).To(Equal(int64(100)))
		})

		It("should return 0 for empty volumeID", func() {
			result := GetVolumeSize(fakeInv, "")

			Expect(result).To(Equal(int64(0)))
		})

		It("should return 0 when volume not found", func() {
			result := GetVolumeSize(fakeInv, "vol-nonexistent")

			Expect(result).To(Equal(int64(0)))
		})

		It("should return 0 when Size is nil", func() {
			volumeDetails := &model.VolumeDetails{}
			volumeDetails.Size = nil
			fakeInv.Volumes["vol-123"] = &web.Volume{
				Resource: web.Resource{ID: "vol-123"},
				Object:   volumeDetails,
			}

			result := GetVolumeSize(fakeInv, "vol-123")

			Expect(result).To(Equal(int64(0)))
		})
	})

	Describe("ParseBlockDevices", func() {
		It("should parse EBS and instance store devices", func() {
			instance := &model.InstanceDetails{
				BlockDeviceMappings: []model.InstanceBlockDeviceMapping{
					{
						DeviceName: aws.String("/dev/sda1"),
						Ebs:        &model.EbsInstanceBlockDevice{VolumeId: aws.String("vol-111")},
					},
					{
						DeviceName: aws.String("/dev/sdb"),
						Ebs:        &model.EbsInstanceBlockDevice{VolumeId: aws.String("vol-222")},
					},
					{
						DeviceName:  aws.String("/dev/sdc"),
						VirtualName: aws.String("ephemeral0"),
					},
				},
			}

			stats := ParseBlockDevices(instance)

			Expect(stats.EBSVolumeIDs).To(ConsistOf("vol-111", "vol-222"))
			Expect(stats.InstanceStoreDev).To(ConsistOf("/dev/sdc"))
			Expect(stats.SkippedCount).To(Equal(0))
		})

		It("should handle empty block devices", func() {
			instance := &model.InstanceDetails{
				BlockDeviceMappings: []model.InstanceBlockDeviceMapping{},
			}

			stats := ParseBlockDevices(instance)

			Expect(stats.EBSVolumeIDs).To(BeEmpty())
			Expect(stats.InstanceStoreDev).To(BeEmpty())
			Expect(stats.SkippedCount).To(Equal(0))
		})

		It("should count skipped devices", func() {
			instance := &model.InstanceDetails{
				BlockDeviceMappings: []model.InstanceBlockDeviceMapping{
					{DeviceName: aws.String("/dev/sda1")}, // No Ebs or VirtualName
				},
			}

			stats := ParseBlockDevices(instance)

			Expect(stats.EBSVolumeIDs).To(BeEmpty())
			Expect(stats.InstanceStoreDev).To(BeEmpty())
			Expect(stats.SkippedCount).To(Equal(1))
		})
	})
})
