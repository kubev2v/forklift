package conversion

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	"github.com/kubev2v/forklift/pkg/virt-v2v/config"
	"github.com/kubev2v/forklift/pkg/virt-v2v/utils"
)

var _ = Describe("Disks", func() {
	var disk *Disk
	var mockCtrl *gomock.Controller
	var mockFileSystem *utils.MockFileSystem

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockFileSystem = utils.NewMockFileSystem(mockCtrl)
	})

	DescribeTable("generate disk name from",
		func(diskNum int, expected string) {
			disk = &Disk{}
			Expect(disk.genName(diskNum)).To(Equal(expected))
		},
		Entry("start of alphabet", 1, "a"),
		Entry("end of alphabet", 26, "z"),
		Entry("two character name start", 27, "aa"),
		Entry("two character name next", 28, "ab"),
		Entry("two character name end", 52, "az"),
		Entry("two character name start with next", 53, "ba"),
		Entry("two character name next", 55, "bc"),
		Entry("two character name end", 702, "zz"),
		Entry("three character name", 754, "abz"),
	)
	DescribeTable("get disk number",
		func(diskPath string, expected int) {
			disk = &Disk{
				Path: diskPath,
			}
			num, err := disk.getDiskNumber()
			Expect(err).ToNot(HaveOccurred())
			Expect(num).To(Equal(expected))
		},
		Entry("block device ending with 0", "/dev/block0", 0),
		Entry("block device ending with 12", "/dev/block12", 12),
		Entry("filesystem ending with 0", "/mnt/disks/disk0", 0),
		Entry("filesystem ending with 13", "/mnt/disks/disk13", 13),
		Entry("filesystem ending with 0 and pointing to the image", "/mnt/disks/disk0/disk.img", 0),
		Entry("filesystem ending with 14 and pointing to the image", "/mnt/disks/disk14/disk.img", 14),
	)
	DescribeTable("create disk link with new vm name",
		func(diskPath string, expected string) {
			cfg := &config.AppConfig{
				Workdir:   "/var/tmp/v2v",
				VmName:    "vm-name",
				NewVmName: "new-vm-name",
			}
			disk := Disk{
				Path:       diskPath,
				appConfig:  cfg,
				fileSystem: mockFileSystem,
			}
			mockFileSystem.EXPECT().Symlink(diskPath, expected)
			link, err := disk.createLink()
			Expect(err).ToNot(HaveOccurred())
			Expect(link).To(Equal(expected))
		},
		Entry("block device ending with 0", "/dev/block0", "/var/tmp/v2v/new-vm-name-sda"),
		Entry("filesystem image ending with 0", "/mnt/disks/disk0/disk.img", "/var/tmp/v2v/new-vm-name-sda"),
		Entry("block device ending with 1", "/dev/block1", "/var/tmp/v2v/new-vm-name-sdb"),
		Entry("filesystem image ending with 1", "/mnt/disks/disk1/disk.img", "/var/tmp/v2v/new-vm-name-sdb"),
	)
	DescribeTable("Extracting the author's first and last name",
		func(diskPath string, expected string) {
			cfg := &config.AppConfig{
				Workdir: "/var/tmp/v2v",
				VmName:  "vm-name",
			}
			disk := Disk{
				Path:       diskPath,
				appConfig:  cfg,
				fileSystem: mockFileSystem,
			}
			mockFileSystem.EXPECT().Symlink(diskPath, expected)
			link, err := disk.createLink()
			Expect(err).ToNot(HaveOccurred())
			Expect(link).To(Equal(expected))
		},
		Entry("block device ending with 0", "/dev/block0", "/var/tmp/v2v/vm-name-sda"),
		Entry("filesystem image ending with 0", "/mnt/disks/disk0/disk.img", "/var/tmp/v2v/vm-name-sda"),
		Entry("block device ending with 1", "/dev/block1", "/var/tmp/v2v/vm-name-sdb"),
		Entry("filesystem image ending with 1", "/mnt/disks/disk1/disk.img", "/var/tmp/v2v/vm-name-sdb"),
		Entry("block device ending with 12", "/dev/block12", "/var/tmp/v2v/vm-name-sdm"),
		Entry("filesystem image ending with 13", "/mnt/disks/disk13/disk.img", "/var/tmp/v2v/vm-name-sdn"),
	)
})
