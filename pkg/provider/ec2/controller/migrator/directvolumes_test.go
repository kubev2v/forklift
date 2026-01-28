package migrator

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/kubev2v/forklift/pkg/provider/ec2/controller/inventory"
	"github.com/kubev2v/forklift/pkg/provider/ec2/inventory/model"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestMigrator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "EC2 Migrator Suite")
}

var _ = Describe("Volume Ordering", func() {
	Describe("BlockDeviceMappings iteration order", func() {
		It("should preserve disk order from BlockDeviceMappings", func() {
			// Create an instance with volumes in a specific order.
			// The key insight is that BlockDeviceMappings is a slice (ordered),
			// while volumeMapping is a map (unordered).
			instance := &model.InstanceDetails{
				BlockDeviceMappings: []model.InstanceBlockDeviceMapping{
					{
						DeviceName: aws.String("/dev/sda1"),
						Ebs:        &model.EbsInstanceBlockDevice{VolumeId: aws.String("vol-aaa")},
					},
					{
						DeviceName: aws.String("/dev/sdb"),
						Ebs:        &model.EbsInstanceBlockDevice{VolumeId: aws.String("vol-bbb")},
					},
					{
						DeviceName: aws.String("/dev/sdc"),
						Ebs:        &model.EbsInstanceBlockDevice{VolumeId: aws.String("vol-ccc")},
					},
					{
						DeviceName: aws.String("/dev/sdd"),
						Ebs:        &model.EbsInstanceBlockDevice{VolumeId: aws.String("vol-ddd")},
					},
				},
			}

			// Simulate the volume mapping that would come from AWS tags.
			// This is a map, so iteration order is non-deterministic.
			volumeMapping := map[string]string{
				"vol-aaa": "vol-new-aaa",
				"vol-bbb": "vol-new-bbb",
				"vol-ccc": "vol-new-ccc",
				"vol-ddd": "vol-new-ddd",
			}

			// Get block devices in order
			blockDevices, found := inventory.GetBlockDevices(instance)
			Expect(found).To(BeTrue())
			Expect(blockDevices).To(HaveLen(4))

			// Simulate the fixed iteration pattern from createPVsAndPVCs.
			// Iterate over BlockDeviceMappings (ordered slice) using slice index.
			var orderedVolumeIDs []string
			var orderedIndices []int
			for i, dev := range blockDevices {
				if dev.Ebs == nil || dev.Ebs.VolumeId == nil {
					continue
				}

				originalVolumeID := *dev.Ebs.VolumeId
				_, found := volumeMapping[originalVolumeID]
				if !found {
					continue
				}

				orderedVolumeIDs = append(orderedVolumeIDs, originalVolumeID)
				orderedIndices = append(orderedIndices, i)
			}

			// Verify the volumes are in the correct order (matching BlockDeviceMappings position)
			Expect(orderedVolumeIDs).To(Equal([]string{"vol-aaa", "vol-bbb", "vol-ccc", "vol-ddd"}))
			Expect(orderedIndices).To(Equal([]int{0, 1, 2, 3}))
		})

		It("should handle missing volumes in volumeMapping gracefully", func() {
			instance := &model.InstanceDetails{
				BlockDeviceMappings: []model.InstanceBlockDeviceMapping{
					{
						DeviceName: aws.String("/dev/sda1"),
						Ebs:        &model.EbsInstanceBlockDevice{VolumeId: aws.String("vol-aaa")},
					},
					{
						DeviceName: aws.String("/dev/sdb"),
						Ebs:        &model.EbsInstanceBlockDevice{VolumeId: aws.String("vol-bbb")},
					},
					{
						DeviceName: aws.String("/dev/sdc"),
						Ebs:        &model.EbsInstanceBlockDevice{VolumeId: aws.String("vol-ccc")},
					},
				},
			}

			// Only some volumes have been migrated
			volumeMapping := map[string]string{
				"vol-aaa": "vol-new-aaa",
				// vol-bbb is missing (not yet migrated)
				"vol-ccc": "vol-new-ccc",
			}

			blockDevices, _ := inventory.GetBlockDevices(instance)

			var orderedVolumeIDs []string
			var orderedIndices []int
			for i, dev := range blockDevices {
				if dev.Ebs == nil || dev.Ebs.VolumeId == nil {
					continue
				}

				originalVolumeID := *dev.Ebs.VolumeId
				_, found := volumeMapping[originalVolumeID]
				if !found {
					continue
				}

				orderedVolumeIDs = append(orderedVolumeIDs, originalVolumeID)
				orderedIndices = append(orderedIndices, i)
			}

			// Only the present volumes should be included, with their original slice positions
			Expect(orderedVolumeIDs).To(Equal([]string{"vol-aaa", "vol-ccc"}))
			Expect(orderedIndices).To(Equal([]int{0, 2})) // positions 0 and 2, skipping position 1 (vol-bbb)
		})

		It("should skip non-EBS block devices", func() {
			instance := &model.InstanceDetails{
				BlockDeviceMappings: []model.InstanceBlockDeviceMapping{
					{
						DeviceName: aws.String("/dev/sda1"),
						Ebs:        &model.EbsInstanceBlockDevice{VolumeId: aws.String("vol-aaa")},
					},
					{
						// Instance store (ephemeral) - no EBS
						DeviceName:  aws.String("/dev/sdb"),
						VirtualName: aws.String("ephemeral0"),
					},
					{
						DeviceName: aws.String("/dev/sdc"),
						Ebs:        &model.EbsInstanceBlockDevice{VolumeId: aws.String("vol-ccc")},
					},
				},
			}

			volumeMapping := map[string]string{
				"vol-aaa": "vol-new-aaa",
				"vol-ccc": "vol-new-ccc",
			}

			blockDevices, _ := inventory.GetBlockDevices(instance)

			var orderedVolumeIDs []string
			var orderedIndices []int
			for i, dev := range blockDevices {
				if dev.Ebs == nil || dev.Ebs.VolumeId == nil {
					continue
				}

				originalVolumeID := *dev.Ebs.VolumeId
				_, found := volumeMapping[originalVolumeID]
				if !found {
					continue
				}

				orderedVolumeIDs = append(orderedVolumeIDs, originalVolumeID)
				orderedIndices = append(orderedIndices, i)
			}

			// Instance store at position 1 is skipped, EBS volumes keep their slice positions
			Expect(orderedVolumeIDs).To(Equal([]string{"vol-aaa", "vol-ccc"}))
			Expect(orderedIndices).To(Equal([]int{0, 2})) // positions 0 and 2, skipping instance store at 1
		})

		It("should handle empty BlockDeviceMappings", func() {
			instance := &model.InstanceDetails{
				BlockDeviceMappings: []model.InstanceBlockDeviceMapping{},
			}

			blockDevices, found := inventory.GetBlockDevices(instance)
			Expect(found).To(BeFalse())
			Expect(blockDevices).To(BeEmpty())
		})
	})
})
