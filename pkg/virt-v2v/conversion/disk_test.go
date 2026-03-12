// Generated-by: Claude
package conversion

import (
	"errors"

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

	Describe("genName", func() {
		DescribeTable("generates disk name correctly",
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

		DescribeTable("handles edge cases",
			func(diskNum int, expected string) {
				disk = &Disk{}
				Expect(disk.genName(diskNum)).To(Equal(expected))
			},
			Entry("zero returns empty string", 0, ""),
			Entry("negative returns empty string", -1, ""),
			Entry("negative large value returns empty string", -100, ""),
		)
	})

	Describe("getDiskNumber", func() {
		DescribeTable("extracts disk number from path",
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
			Entry("large disk number", "/dev/block999", 999),
			Entry("path with multiple numbers takes first", "/mnt/disk1/disk2/disk.img", 1),
		)

		It("returns error for path without number",
			func() {
				disk = &Disk{
					Path: "/dev/sda",
				}
				_, err := disk.getDiskNumber()
				Expect(err).To(HaveOccurred())
			},
		)

		It("returns error for empty path",
			func() {
				disk = &Disk{
					Path: "",
				}
				_, err := disk.getDiskNumber()
				Expect(err).To(HaveOccurred())
			},
		)
	})

	Describe("getDiskName", func() {
		It("returns NewVmName when set",
			func() {
				cfg := &config.AppConfig{
					VmName:    "original-vm",
					NewVmName: "new-vm",
				}
				disk = &Disk{
					appConfig: cfg,
				}
				Expect(disk.getDiskName()).To(Equal("new-vm"))
			},
		)

		It("returns VmName when NewVmName is not set",
			func() {
				cfg := &config.AppConfig{
					VmName:    "original-vm",
					NewVmName: "",
				}
				disk = &Disk{
					appConfig: cfg,
				}
				Expect(disk.getDiskName()).To(Equal("original-vm"))
			},
		)

		It("returns empty string when both are empty",
			func() {
				cfg := &config.AppConfig{
					VmName:    "",
					NewVmName: "",
				}
				disk = &Disk{
					appConfig: cfg,
				}
				Expect(disk.getDiskName()).To(Equal(""))
			},
		)
	})

	Describe("createLink", func() {
		DescribeTable("creates disk link with new vm name",
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

		DescribeTable("creates disk link with original vm name",
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

		It("returns error when symlink fails",
			func() {
				cfg := &config.AppConfig{
					Workdir: "/var/tmp/v2v",
					VmName:  "vm-name",
				}
				disk := Disk{
					Path:       "/dev/block0",
					appConfig:  cfg,
					fileSystem: mockFileSystem,
				}
				mockFileSystem.EXPECT().Symlink("/dev/block0", "/var/tmp/v2v/vm-name-sda").Return(errors.New("symlink failed"))

				_, err := disk.createLink()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("symlink failed"))
			},
		)

		It("returns error when disk number extraction fails",
			func() {
				cfg := &config.AppConfig{
					Workdir: "/var/tmp/v2v",
					VmName:  "vm-name",
				}
				disk := Disk{
					Path:       "/dev/sda", // No number in path
					appConfig:  cfg,
					fileSystem: mockFileSystem,
				}

				_, err := disk.createLink()
				Expect(err).To(HaveOccurred())
			},
		)

		DescribeTable("creates correct link for high disk numbers",
			func(diskPath string, expected string) {
				cfg := &config.AppConfig{
					Workdir: "/var/tmp/v2v",
					VmName:  "vm",
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
			Entry("disk 25 -> sdz", "/dev/block25", "/var/tmp/v2v/vm-sdz"),
			Entry("disk 26 -> sdaa", "/dev/block26", "/var/tmp/v2v/vm-sdaa"),
			Entry("disk 27 -> sdab", "/dev/block27", "/var/tmp/v2v/vm-sdab"),
			Entry("disk 51 -> sdaz", "/dev/block51", "/var/tmp/v2v/vm-sdaz"),
			Entry("disk 52 -> sdba", "/dev/block52", "/var/tmp/v2v/vm-sdba"),
		)
	})

})
