// Generated-by: Claude
package conversion

import (
	"errors"
	"os"
	"testing"

	"github.com/kubev2v/forklift/pkg/virt-v2v/config"
	"github.com/kubev2v/forklift/pkg/virt-v2v/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

func TestConversion(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Conversion test suite")
}

var _ = Describe("Conversion", func() {
	var conversion *Conversion
	var mockCtrl *gomock.Controller
	var mockCommandExecutor *utils.MockCommandExecutor
	var mockCommandBuilder *utils.MockCommandBuilder
	var mockFileSystem *utils.MockFileSystem
	var appConfig *config.AppConfig

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockCommandExecutor = utils.NewMockCommandExecutor(mockCtrl)
		mockCommandBuilder = utils.NewMockCommandBuilder(mockCtrl)
		mockFileSystem = utils.NewMockFileSystem(mockCtrl)

		appConfig = &config.AppConfig{}
		conversion = &Conversion{
			AppConfig:      appConfig,
			CommandBuilder: mockCommandBuilder,
			fileSystem:     mockFileSystem,
		}
	})

	Describe("RunVirtV2VInspection", func() {
		It("passes virt-v2v-inspection with multiple disks",
			func() {
				appConfig.InspectionOutputFile = config.InspectionOutputFile
				conversion.Disks = []*Disk{
					{Link: "/var/tmp/v2v/new-vm-name-sda"},
					{Link: "/var/tmp/v2v/new-vm-name-sdb"},
				}

				mockCommandBuilder.EXPECT().New("virt-v2v-inspector").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddFlag("-v").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddFlag("-x").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddArg("--root", "first").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddArg("-if", "raw").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddArg("-i", "disk").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddArg("-O", config.InspectionOutputFile).Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddPositional("/var/tmp/v2v/new-vm-name-sda").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddPositional("/var/tmp/v2v/new-vm-name-sdb").Return(mockCommandBuilder)

				mockCommandBuilder.EXPECT().Build().Return(mockCommandExecutor)

				mockCommandExecutor.EXPECT().SetStdout(os.Stdout)
				mockCommandExecutor.EXPECT().SetStderr(os.Stderr)
				mockCommandExecutor.EXPECT().Run()

				err := conversion.RunVirtV2VInspection()
				Expect(err).ToNot(HaveOccurred())
			},
		)

		It("passes virt-v2v-inspection with inspector extra args",
			func() {
				appConfig.InspectionOutputFile = config.InspectionOutputFile
				appConfig.InspectorExtraArgs = []string{"--extra-arg1", "--extra-arg2"}
				conversion.Disks = []*Disk{
					{Link: "/var/tmp/v2v/new-vm-name-sda"},
				}

				mockCommandBuilder.EXPECT().New("virt-v2v-inspector").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddFlag("-v").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddFlag("-x").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddArg("--root", "first").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddArg("-if", "raw").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddArg("-i", "disk").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddArg("-O", config.InspectionOutputFile).Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddExtraArgs("--extra-arg1", "--extra-arg2").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddPositional("/var/tmp/v2v/new-vm-name-sda").Return(mockCommandBuilder)

				mockCommandBuilder.EXPECT().Build().Return(mockCommandExecutor)

				mockCommandExecutor.EXPECT().SetStdout(os.Stdout)
				mockCommandExecutor.EXPECT().SetStderr(os.Stderr)
				mockCommandExecutor.EXPECT().Run()

				err := conversion.RunVirtV2VInspection()
				Expect(err).ToNot(HaveOccurred())
			},
		)

		It("returns error when command fails",
			func() {
				appConfig.InspectionOutputFile = config.InspectionOutputFile
				conversion.Disks = []*Disk{
					{Link: "/var/tmp/v2v/new-vm-name-sda"},
				}

				mockCommandBuilder.EXPECT().New("virt-v2v-inspector").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddFlag("-v").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddFlag("-x").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddArg("--root", "first").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddArg("-if", "raw").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddArg("-i", "disk").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddArg("-O", config.InspectionOutputFile).Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddPositional("/var/tmp/v2v/new-vm-name-sda").Return(mockCommandBuilder)

				mockCommandBuilder.EXPECT().Build().Return(mockCommandExecutor)

				mockCommandExecutor.EXPECT().SetStdout(os.Stdout)
				mockCommandExecutor.EXPECT().SetStderr(os.Stderr)
				mockCommandExecutor.EXPECT().Run().Return(errors.New("command failed"))

				err := conversion.RunVirtV2VInspection()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("command failed"))
			},
		)
	})

	Describe("RunVirtV2vInPlace", func() {
		It("passes virt-v2v-in-place with libvirtxml",
			func() {
				appConfig.LibvirtDomainFile = config.V2vInPlaceLibvirtDomain

				mockCommandBuilder.EXPECT().New("virt-v2v-in-place").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddFlag("-v").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddFlag("-x").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddArg("--root", "first").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddArg("-i", "libvirtxml").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddPositional(config.V2vInPlaceLibvirtDomain).Return(mockCommandBuilder)

				mockCommandBuilder.EXPECT().Build().Return(mockCommandExecutor)

				mockCommandExecutor.EXPECT().SetStdout(os.Stdout)
				mockCommandExecutor.EXPECT().SetStderr(os.Stderr)
				mockCommandExecutor.EXPECT().Run()

				err := conversion.RunVirtV2vInPlace()
				Expect(err).ToNot(HaveOccurred())
			},
		)

		It("passes virt-v2v-in-place with extra args",
			func() {
				appConfig.LibvirtDomainFile = config.V2vInPlaceLibvirtDomain
				appConfig.ExtraArgs = []string{"--debug", "--verbose"}

				mockCommandBuilder.EXPECT().New("virt-v2v-in-place").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddFlag("-v").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddFlag("-x").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddArg("--root", "first").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddArg("-i", "libvirtxml").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddExtraArgs("--debug", "--verbose").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddPositional(config.V2vInPlaceLibvirtDomain).Return(mockCommandBuilder)

				mockCommandBuilder.EXPECT().Build().Return(mockCommandExecutor)

				mockCommandExecutor.EXPECT().SetStdout(os.Stdout)
				mockCommandExecutor.EXPECT().SetStderr(os.Stderr)
				mockCommandExecutor.EXPECT().Run()

				err := conversion.RunVirtV2vInPlace()
				Expect(err).ToNot(HaveOccurred())
			},
		)

		It("returns error when command fails",
			func() {
				appConfig.LibvirtDomainFile = config.V2vInPlaceLibvirtDomain

				mockCommandBuilder.EXPECT().New("virt-v2v-in-place").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddFlag("-v").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddFlag("-x").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddArg("--root", "first").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddArg("-i", "libvirtxml").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddPositional(config.V2vInPlaceLibvirtDomain).Return(mockCommandBuilder)

				mockCommandBuilder.EXPECT().Build().Return(mockCommandExecutor)

				mockCommandExecutor.EXPECT().SetStdout(os.Stdout)
				mockCommandExecutor.EXPECT().SetStderr(os.Stderr)
				mockCommandExecutor.EXPECT().Run().Return(errors.New("in-place conversion failed"))

				err := conversion.RunVirtV2vInPlace()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("in-place conversion failed"))
			},
		)
	})

	Describe("RunVirtV2vInPlaceDisk", func() {
		It("runs virt-v2v-in-place with disk mode",
			func() {
				conversion.Disks = []*Disk{
					{Link: "/var/tmp/v2v/vm-sda"},
					{Link: "/var/tmp/v2v/vm-sdb"},
				}

				mockCommandBuilder.EXPECT().New("virt-v2v-in-place").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddFlag("-v").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddFlag("-x").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddArg("-i", "disk").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddArg("--root", "first").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddPositional("/var/tmp/v2v/vm-sda").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddPositional("/var/tmp/v2v/vm-sdb").Return(mockCommandBuilder)

				mockCommandBuilder.EXPECT().Build().Return(mockCommandExecutor)

				mockCommandExecutor.EXPECT().SetStdout(os.Stdout)
				mockCommandExecutor.EXPECT().SetStderr(os.Stderr)
				mockCommandExecutor.EXPECT().Run()

				err := conversion.RunVirtV2vInPlaceDisk()
				Expect(err).ToNot(HaveOccurred())
			},
		)

		It("returns error when no disks found",
			func() {
				conversion.Disks = []*Disk{}

				err := conversion.RunVirtV2vInPlaceDisk()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("no disks found for in-place conversion"))
			},
		)

		It("runs virt-v2v-in-place disk mode with extra args",
			func() {
				appConfig.ExtraArgs = []string{"--custom-flag"}
				conversion.Disks = []*Disk{
					{Link: "/var/tmp/v2v/vm-sda"},
				}

				mockCommandBuilder.EXPECT().New("virt-v2v-in-place").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddFlag("-v").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddFlag("-x").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddArg("-i", "disk").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddArg("--root", "first").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddExtraArgs("--custom-flag").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddPositional("/var/tmp/v2v/vm-sda").Return(mockCommandBuilder)

				mockCommandBuilder.EXPECT().Build().Return(mockCommandExecutor)

				mockCommandExecutor.EXPECT().SetStdout(os.Stdout)
				mockCommandExecutor.EXPECT().SetStderr(os.Stderr)
				mockCommandExecutor.EXPECT().Run()

				err := conversion.RunVirtV2vInPlaceDisk()
				Expect(err).ToNot(HaveOccurred())
			},
		)
	})

	Describe("addCommonArgs", func() {
		It("adds common args with root disk and multiple static IPs",
			func() {
				appConfig := config.AppConfig{
					RootDisk:  "/dev/sda",
					StaticIPs: "00:11:22:33:44:55:ip:192.168.1.100_00:11:22:33:44:56:ip:192.168.1.101",
				}
				conversion.AppConfig = &appConfig

				mockCommandBuilder.EXPECT().AddArg("--root", "/dev/sda").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddArg("--mac", "00:11:22:33:44:55:ip:192.168.1.100").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddArg("--mac", "00:11:22:33:44:56:ip:192.168.1.101").Return(mockCommandBuilder)

				err := conversion.addCommonArgs(mockCommandBuilder)
				Expect(err).ToNot(HaveOccurred())
			},
		)

		It("adds common args with root disk and single static IP",
			func() {
				appConfig := config.AppConfig{
					RootDisk:  "/dev/sda",
					StaticIPs: "00:11:22:33:44:55:ip:192.168.1.100",
				}
				conversion.AppConfig = &appConfig
				mockCommandBuilder.EXPECT().AddArg("--root", "/dev/sda").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddArg("--mac", "00:11:22:33:44:55:ip:192.168.1.100").Return(mockCommandBuilder)

				err := conversion.addCommonArgs(mockCommandBuilder)
				Expect(err).ToNot(HaveOccurred())
			},
		)

		It("adds common args with root disk as default",
			func() {
				appConfig := config.AppConfig{}
				conversion = &Conversion{
					AppConfig:      &appConfig,
					CommandBuilder: mockCommandBuilder,
				}
				mockCommandBuilder.EXPECT().AddArg("--root", "first").Return(mockCommandBuilder)

				err := conversion.addCommonArgs(mockCommandBuilder)
				Expect(err).ToNot(HaveOccurred())
			},
		)

		It("adds clevis key when NbdeClevis is true",
			func() {
				appConfig := config.AppConfig{
					NbdeClevis: true,
				}
				conversion.AppConfig = &appConfig

				mockCommandBuilder.EXPECT().AddArg("--root", "first").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddArgs("--key", "all:clevis").Return(mockCommandBuilder)

				err := conversion.addCommonArgs(mockCommandBuilder)
				Expect(err).ToNot(HaveOccurred())
			},
		)

		It("adds LUKS keys when Luksdir is set and files exist",
			func() {
				luksDir := "/etc/luks"
				appConfig := config.AppConfig{
					Luksdir: luksDir,
				}
				conversion.AppConfig = &appConfig

				// Mock filesystem.Stat to indicate directory exists
				mockFileSystem.EXPECT().Stat(luksDir).Return(nil, nil)
				// Mock filesystem to return empty directory (no LUKS keys)
				mockFileSystem.EXPECT().ReadDir(luksDir).Return([]os.DirEntry{}, nil)

				mockCommandBuilder.EXPECT().AddArg("--root", "first").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddArgs("--key").Return(mockCommandBuilder)

				err := conversion.addCommonArgs(mockCommandBuilder)
				Expect(err).ToNot(HaveOccurred())
			},
		)
	})

	Describe("addConversionExtraArgs", func() {
		It("adds extra args when they are set",
			func() {
				appConfig.ExtraArgs = []string{"--arg1", "--arg2", "value"}

				mockCommandBuilder.EXPECT().AddExtraArgs("--arg1", "--arg2", "value").Return(mockCommandBuilder)

				conversion.addConversionExtraArgs(mockCommandBuilder)
			},
		)

		It("does nothing when extra args are nil",
			func() {
				appConfig.ExtraArgs = nil
				// No mock expectations - nothing should be called
				conversion.addConversionExtraArgs(mockCommandBuilder)
			},
		)

		It("calls AddExtraArgs with empty slice when extra args are empty",
			func() {
				appConfig.ExtraArgs = []string{}
				// Empty slice is not nil, so AddExtraArgs is still called
				mockCommandBuilder.EXPECT().AddExtraArgs().Return(mockCommandBuilder)
				conversion.addConversionExtraArgs(mockCommandBuilder)
			},
		)
	})

	Describe("addInspectorExtraArgs", func() {
		It("adds inspector extra args when they are set",
			func() {
				appConfig.InspectorExtraArgs = []string{"--inspector-arg1", "--inspector-arg2"}

				mockCommandBuilder.EXPECT().AddExtraArgs("--inspector-arg1", "--inspector-arg2").Return(mockCommandBuilder)

				conversion.addInspectorExtraArgs(mockCommandBuilder)
			},
		)

		It("does nothing when inspector extra args are nil",
			func() {
				appConfig.InspectorExtraArgs = nil
				// No mock expectations - nothing should be called
				conversion.addInspectorExtraArgs(mockCommandBuilder)
			},
		)
	})

	Describe("virtV2vOVAArgs", func() {
		It("adds OVA args correctly",
			func() {
				appConfig.DiskPath = "/path/to/ova/disk.ova"

				mockCommandBuilder.EXPECT().AddArg("-i", "ova").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddPositional("/path/to/ova/disk.ova").Return(mockCommandBuilder)

				conversion.virtV2vOVAArgs(mockCommandBuilder)
			},
		)
	})

	Describe("addVirtV2vRemoteInspectionArgs", func() {
		It("adds remote inspection disk args",
			func() {
				appConfig.RemoteInspectionDisks = []string{"[datastore1] vm/disk1.vmdk", "[datastore1] vm/disk2.vmdk"}

				mockCommandBuilder.EXPECT().AddArg("-io", "vddk-file=[datastore1] vm/disk1.vmdk").Return(mockCommandBuilder)
				mockCommandBuilder.EXPECT().AddArg("-io", "vddk-file=[datastore1] vm/disk2.vmdk").Return(mockCommandBuilder)

				err := conversion.addVirtV2vRemoteInspectionArgs(mockCommandBuilder)
				Expect(err).ToNot(HaveOccurred())
			},
		)

		It("returns error when no remote disks are supplied",
			func() {
				appConfig.RemoteInspectionDisks = []string{}

				err := conversion.addVirtV2vRemoteInspectionArgs(mockCommandBuilder)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("No remote disks were supplied"))
			},
		)
	})

	Describe("RunVirtV2vInPlaceDisk error cases", func() {
		It("returns error when command fails", func() {
			conversion.Disks = []*Disk{
				{Link: "/var/tmp/v2v/vm-sda"},
			}

			mockCommandBuilder.EXPECT().New("virt-v2v-in-place").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddFlag("-v").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddFlag("-x").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("-i", "disk").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("--root", "first").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddPositional("/var/tmp/v2v/vm-sda").Return(mockCommandBuilder)

			mockCommandBuilder.EXPECT().Build().Return(mockCommandExecutor)

			mockCommandExecutor.EXPECT().SetStdout(os.Stdout)
			mockCommandExecutor.EXPECT().SetStderr(os.Stderr)
			mockCommandExecutor.EXPECT().Run().Return(errors.New("disk conversion failed"))

			err := conversion.RunVirtV2vInPlaceDisk()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("disk conversion failed"))
		})

		It("returns error when addCommonArgs fails", func() {
			luksDir := "/etc/luks"
			appConfig.Luksdir = luksDir
			conversion.Disks = []*Disk{
				{Link: "/var/tmp/v2v/vm-sda"},
			}

			mockCommandBuilder.EXPECT().New("virt-v2v-in-place").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddFlag("-v").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddFlag("-x").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("-i", "disk").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("--root", "first").Return(mockCommandBuilder)

			mockFileSystem.EXPECT().Stat(luksDir).Return(nil, nil)
			mockFileSystem.EXPECT().ReadDir(luksDir).Return(nil, errors.New("permission denied"))

			err := conversion.RunVirtV2vInPlaceDisk()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("error adding LUKS keys"))
		})
	})

	Describe("RunVirtV2VInspection with custom root disk", func() {
		It("passes custom root disk argument", func() {
			appConfig.InspectionOutputFile = config.InspectionOutputFile
			appConfig.RootDisk = "/dev/sdb"
			conversion.Disks = []*Disk{
				{Link: "/var/tmp/v2v/vm-sda"},
			}

			mockCommandBuilder.EXPECT().New("virt-v2v-inspector").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddFlag("-v").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddFlag("-x").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("-if", "raw").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("-i", "disk").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("-O", config.InspectionOutputFile).Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("--root", "/dev/sdb").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddPositional("/var/tmp/v2v/vm-sda").Return(mockCommandBuilder)

			mockCommandBuilder.EXPECT().Build().Return(mockCommandExecutor)

			mockCommandExecutor.EXPECT().SetStdout(os.Stdout)
			mockCommandExecutor.EXPECT().SetStderr(os.Stderr)
			mockCommandExecutor.EXPECT().Run()

			err := conversion.RunVirtV2VInspection()
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("RunVirtV2vInPlace with static IPs", func() {
		It("passes static IP arguments", func() {
			appConfig.LibvirtDomainFile = config.V2vInPlaceLibvirtDomain
			appConfig.StaticIPs = "00:11:22:33:44:55:ip:192.168.1.100"

			mockCommandBuilder.EXPECT().New("virt-v2v-in-place").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddFlag("-v").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddFlag("-x").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("-i", "libvirtxml").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("--root", "first").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("--mac", "00:11:22:33:44:55:ip:192.168.1.100").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddPositional(config.V2vInPlaceLibvirtDomain).Return(mockCommandBuilder)

			mockCommandBuilder.EXPECT().Build().Return(mockCommandExecutor)

			mockCommandExecutor.EXPECT().SetStdout(os.Stdout)
			mockCommandExecutor.EXPECT().SetStderr(os.Stderr)
			mockCommandExecutor.EXPECT().Run()

			err := conversion.RunVirtV2vInPlace()
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns error when addCommonArgs fails", func() {
			appConfig.LibvirtDomainFile = config.V2vInPlaceLibvirtDomain
			luksDir := "/etc/luks"
			appConfig.Luksdir = luksDir

			mockCommandBuilder.EXPECT().New("virt-v2v-in-place").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddFlag("-v").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddFlag("-x").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("-i", "libvirtxml").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("--root", "first").Return(mockCommandBuilder)

			mockFileSystem.EXPECT().Stat(luksDir).Return(nil, nil)
			mockFileSystem.EXPECT().ReadDir(luksDir).Return(nil, errors.New("read error"))

			err := conversion.RunVirtV2vInPlace()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("error adding LUKS keys"))
		})
	})

	Describe("addVirtV2vArgs", func() {
		It("adds OVA args when source is OVA", func() {
			appConfig.Source = config.OVA
			appConfig.Workdir = "/var/tmp/v2v"
			appConfig.NewVmName = "new-vm"
			appConfig.DiskPath = "/path/to/disk.ova"

			mockCommandBuilder.EXPECT().AddFlag("-v").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddFlag("-x").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("-o", "kubevirt").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("-os", "/var/tmp/v2v").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("-on", "new-vm").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("-i", "ova").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddPositional("/path/to/disk.ova").Return(mockCommandBuilder)

			err := conversion.addVirtV2vArgs(mockCommandBuilder)
			Expect(err).ToNot(HaveOccurred())
		})

		It("handles unknown source gracefully", func() {
			appConfig.Source = "unknown"
			appConfig.Workdir = "/var/tmp/v2v"
			appConfig.NewVmName = "new-vm"

			mockCommandBuilder.EXPECT().AddFlag("-v").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddFlag("-x").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("-o", "kubevirt").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("-os", "/var/tmp/v2v").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("-on", "new-vm").Return(mockCommandBuilder)

			err := conversion.addVirtV2vArgs(mockCommandBuilder)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("addCommonArgs with LUKS files", func() {
		It("adds LUKS keys from files in directory", func() {
			luksDir := "/etc/luks"
			appConfig := config.AppConfig{
				Luksdir: luksDir,
			}
			conversion.AppConfig = &appConfig

			luksFiles := utils.ConvertMockDirEntryToOs([]utils.MockDirEntry{
				{FileName: "key1", FileIsDir: false},
				{FileName: "key2", FileIsDir: false},
			})

			mockFileSystem.EXPECT().Stat(luksDir).Return(nil, nil)
			mockFileSystem.EXPECT().ReadDir(luksDir).Return(luksFiles, nil)

			mockCommandBuilder.EXPECT().AddArg("--root", "first").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArgs("--key", "all:file:/etc/luks/key1", "all:file:/etc/luks/key2").Return(mockCommandBuilder)

			err := conversion.addCommonArgs(mockCommandBuilder)
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns error when LUKS directory read fails", func() {
			luksDir := "/etc/luks"
			appConfig := config.AppConfig{
				Luksdir: luksDir,
			}
			conversion.AppConfig = &appConfig

			mockFileSystem.EXPECT().Stat(luksDir).Return(nil, nil)
			mockFileSystem.EXPECT().ReadDir(luksDir).Return(nil, errors.New("permission denied"))

			mockCommandBuilder.EXPECT().AddArg("--root", "first").Return(mockCommandBuilder)

			err := conversion.addCommonArgs(mockCommandBuilder)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("error adding LUKS keys"))
		})
	})

	Describe("addVirtV2vRemoteInspectionArgs with multiple disks", func() {
		It("adds all remote inspection disk args", func() {
			appConfig.RemoteInspectionDisks = []string{
				"[datastore1] vm/disk1.vmdk",
				"[datastore1] vm/disk2.vmdk",
				"[datastore2] vm/disk3.vmdk",
			}

			mockCommandBuilder.EXPECT().AddArg("-io", "vddk-file=[datastore1] vm/disk1.vmdk").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("-io", "vddk-file=[datastore1] vm/disk2.vmdk").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("-io", "vddk-file=[datastore2] vm/disk3.vmdk").Return(mockCommandBuilder)

			err := conversion.addVirtV2vRemoteInspectionArgs(mockCommandBuilder)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("updateDiskPaths", func() {
		It("updates file source disk paths in domain XML",
			func() {
				conversion.Disks = []*Disk{
					{Link: "/var/tmp/v2v/vm-sda"},
				}

				domainXML := `<domain type='kvm'>
  <name>test-vm</name>
  <devices>
    <disk type='file' device='disk'>
      <source file='/original/path/disk.vmdk'/>
      <target dev='sda' bus='scsi'/>
    </disk>
  </devices>
</domain>`

				result, err := conversion.updateDiskPaths(domainXML)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("/var/tmp/v2v/vm-sda"))
				Expect(result).ToNot(ContainSubstring("/original/path/disk.vmdk"))
			},
		)

		It("updates block source disk paths in domain XML",
			func() {
				conversion.Disks = []*Disk{
					{Link: "/var/tmp/v2v/vm-sda"},
				}

				domainXML := `<domain type='kvm'>
  <name>test-vm</name>
  <devices>
    <disk type='block' device='disk'>
      <source dev='/dev/original-block'/>
      <target dev='sda' bus='scsi'/>
    </disk>
  </devices>
</domain>`

				result, err := conversion.updateDiskPaths(domainXML)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("/var/tmp/v2v/vm-sda"))
				Expect(result).ToNot(ContainSubstring("/dev/original-block"))
			},
		)

		It("handles multiple disks",
			func() {
				conversion.Disks = []*Disk{
					{Link: "/var/tmp/v2v/vm-sda"},
					{Link: "/var/tmp/v2v/vm-sdb"},
				}

				domainXML := `<domain type='kvm'>
  <name>test-vm</name>
  <devices>
    <disk type='file' device='disk'>
      <source file='/original/path/disk1.vmdk'/>
      <target dev='sda' bus='scsi'/>
    </disk>
    <disk type='file' device='disk'>
      <source file='/original/path/disk2.vmdk'/>
      <target dev='sdb' bus='scsi'/>
    </disk>
  </devices>
</domain>`

				result, err := conversion.updateDiskPaths(domainXML)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("/var/tmp/v2v/vm-sda"))
				Expect(result).To(ContainSubstring("/var/tmp/v2v/vm-sdb"))
			},
		)

		It("handles empty devices section",
			func() {
				conversion.Disks = []*Disk{
					{Link: "/var/tmp/v2v/vm-sda"},
				}

				domainXML := `<domain type='kvm'>
  <name>test-vm</name>
  <devices>
  </devices>
</domain>`

				result, err := conversion.updateDiskPaths(domainXML)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(ContainSubstring("/var/tmp/v2v/vm-sda"))
			},
		)

		It("returns error for invalid XML",
			func() {
				conversion.Disks = []*Disk{
					{Link: "/var/tmp/v2v/vm-sda"},
				}

				invalidXML := `<domain type='kvm'>
  <name>test-vm</name>
  <devices>
    <disk type='file' device='disk'>
      <source file='/original/path/disk.vmdk'/>
    <!-- Missing closing tags -->`

				_, err := conversion.updateDiskPaths(invalidXML)
				Expect(err).To(HaveOccurred())
			},
		)

		It("handles more disks in XML than available",
			func() {
				conversion.Disks = []*Disk{
					{Link: "/var/tmp/v2v/vm-sda"},
				}

				domainXML := `<domain type='kvm'>
  <name>test-vm</name>
  <devices>
    <disk type='file' device='disk'>
      <source file='/original/path/disk1.vmdk'/>
      <target dev='sda' bus='scsi'/>
    </disk>
    <disk type='file' device='disk'>
      <source file='/original/path/disk2.vmdk'/>
      <target dev='sdb' bus='scsi'/>
    </disk>
  </devices>
</domain>`

				result, err := conversion.updateDiskPaths(domainXML)
				Expect(err).ToNot(HaveOccurred())
				// First disk should be updated
				Expect(result).To(ContainSubstring("/var/tmp/v2v/vm-sda"))
				// Extra disks beyond available are removed from output
				Expect(result).ToNot(ContainSubstring("/original/path/disk2.vmdk"))
				Expect(result).ToNot(ContainSubstring("sdb"))
			},
		)

		It("skips cdrom devices and updates only regular disks",
			func() {
				conversion.Disks = []*Disk{
					{Link: "/var/tmp/v2v/vm-sda"},
				}

				domainXML := `<domain type='kvm'>
  <name>test-vm</name>
  <devices>
    <disk type='file' device='cdrom'>
      <source file='/original/path/cdrom.iso'/>
      <target dev='hda' bus='ide'/>
    </disk>
    <disk type='file' device='disk'>
      <source file='/original/path/disk1.vmdk'/>
      <target dev='sda' bus='scsi'/>
    </disk>
  </devices>
</domain>`

				result, err := conversion.updateDiskPaths(domainXML)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("/var/tmp/v2v/vm-sda"))
				Expect(result).ToNot(ContainSubstring("cdrom.iso"))
				Expect(result).ToNot(ContainSubstring("cdrom"))
			},
		)

		It("skips cdrom between regular disks without affecting disk index",
			func() {
				conversion.Disks = []*Disk{
					{Link: "/var/tmp/v2v/vm-sda"},
					{Link: "/var/tmp/v2v/vm-sdb"},
				}

				domainXML := `<domain type='kvm'>
  <name>test-vm</name>
  <devices>
    <disk type='file' device='disk'>
      <source file='/original/path/disk1.vmdk'/>
      <target dev='sda' bus='scsi'/>
    </disk>
    <disk type='file' device='cdrom'>
      <source file='/original/path/cdrom.iso'/>
      <target dev='hda' bus='ide'/>
    </disk>
    <disk type='file' device='disk'>
      <source file='/original/path/disk2.vmdk'/>
      <target dev='sdb' bus='scsi'/>
    </disk>
  </devices>
</domain>`

				result, err := conversion.updateDiskPaths(domainXML)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("/var/tmp/v2v/vm-sda"))
				Expect(result).To(ContainSubstring("/var/tmp/v2v/vm-sdb"))
				Expect(result).ToNot(ContainSubstring("cdrom.iso"))
				Expect(result).ToNot(ContainSubstring("cdrom"))
				Expect(result).ToNot(ContainSubstring("/original/path/disk1.vmdk"))
				Expect(result).ToNot(ContainSubstring("/original/path/disk2.vmdk"))
			},
		)

		It("handles only cdrom devices with no regular disks",
			func() {
				conversion.Disks = []*Disk{}

				domainXML := `<domain type='kvm'>
  <name>test-vm</name>
  <devices>
    <disk type='file' device='cdrom'>
      <source file='/original/path/cdrom1.iso'/>
      <target dev='hda' bus='ide'/>
    </disk>
    <disk type='file' device='cdrom'>
      <source file='/original/path/cdrom2.iso'/>
      <target dev='hdb' bus='ide'/>
    </disk>
  </devices>
</domain>`

				result, err := conversion.updateDiskPaths(domainXML)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(ContainSubstring("cdrom1.iso"))
				Expect(result).ToNot(ContainSubstring("cdrom2.iso"))
				Expect(result).ToNot(ContainSubstring("cdrom"))
			},
		)

		It("skips cdrom at the end after regular disks",
			func() {
				conversion.Disks = []*Disk{
					{Link: "/var/tmp/v2v/vm-sda"},
				}

				domainXML := `<domain type='kvm'>
  <name>test-vm</name>
  <devices>
    <disk type='file' device='disk'>
      <source file='/original/path/disk1.vmdk'/>
      <target dev='sda' bus='scsi'/>
    </disk>
    <disk type='file' device='cdrom'>
      <source file='/original/path/cdrom.iso'/>
      <target dev='hda' bus='ide'/>
    </disk>
  </devices>
</domain>`

				result, err := conversion.updateDiskPaths(domainXML)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("/var/tmp/v2v/vm-sda"))
				Expect(result).ToNot(ContainSubstring("cdrom.iso"))
				Expect(result).ToNot(ContainSubstring("cdrom"))
			},
		)

		It("skips multiple cdroms interspersed with multiple disks",
			func() {
				conversion.Disks = []*Disk{
					{Link: "/var/tmp/v2v/vm-sda"},
					{Link: "/var/tmp/v2v/vm-sdb"},
					{Link: "/var/tmp/v2v/vm-sdc"},
				}

				domainXML := `<domain type='kvm'>
  <name>test-vm</name>
  <devices>
    <disk type='file' device='cdrom'>
      <source file='/original/path/cdrom1.iso'/>
      <target dev='hda' bus='ide'/>
    </disk>
    <disk type='file' device='disk'>
      <source file='/original/path/disk1.vmdk'/>
      <target dev='sda' bus='scsi'/>
    </disk>
    <disk type='file' device='cdrom'>
      <source file='/original/path/cdrom2.iso'/>
      <target dev='hdb' bus='ide'/>
    </disk>
    <disk type='file' device='disk'>
      <source file='/original/path/disk2.vmdk'/>
      <target dev='sdb' bus='scsi'/>
    </disk>
    <disk type='file' device='disk'>
      <source file='/original/path/disk3.vmdk'/>
      <target dev='sdc' bus='scsi'/>
    </disk>
    <disk type='file' device='cdrom'>
      <source file='/original/path/cdrom3.iso'/>
      <target dev='hdc' bus='ide'/>
    </disk>
  </devices>
</domain>`

				result, err := conversion.updateDiskPaths(domainXML)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("/var/tmp/v2v/vm-sda"))
				Expect(result).To(ContainSubstring("/var/tmp/v2v/vm-sdb"))
				Expect(result).To(ContainSubstring("/var/tmp/v2v/vm-sdc"))
				Expect(result).ToNot(ContainSubstring("cdrom"))
				Expect(result).ToNot(ContainSubstring(".iso"))
			},
		)

		It("handles more XML disks than available when cdroms are present",
			func() {
				conversion.Disks = []*Disk{
					{Link: "/var/tmp/v2v/vm-sda"},
				}

				domainXML := `<domain type='kvm'>
  <name>test-vm</name>
  <devices>
    <disk type='file' device='cdrom'>
      <source file='/original/path/cdrom.iso'/>
      <target dev='hda' bus='ide'/>
    </disk>
    <disk type='file' device='disk'>
      <source file='/original/path/disk1.vmdk'/>
      <target dev='sda' bus='scsi'/>
    </disk>
    <disk type='file' device='disk'>
      <source file='/original/path/disk2.vmdk'/>
      <target dev='sdb' bus='scsi'/>
    </disk>
  </devices>
</domain>`

				result, err := conversion.updateDiskPaths(domainXML)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("/var/tmp/v2v/vm-sda"))
				Expect(result).ToNot(ContainSubstring("cdrom"))
				Expect(result).ToNot(ContainSubstring("/original/path/disk2.vmdk"))
			},
		)
	})

	Describe("addVirtV2vVsphereArgsForInspection", func() {
		// Auto generated tests for addVirtV2vVsphereArgsForInspection
		// Generated by: claude-4-5-opus
		// Reviewed and uploaded by: @yaacov

		It("adds basic vSphere args without VDDK", func() {
			appConfig.LibvirtUrl = "vpx://user@vcenter.example.com/Datacenter/Cluster/esxi-host?no_verify=1"
			appConfig.SecretKey = "/etc/secret/secretKey"
			appConfig.HostName = "vcenter.example.com"
			appConfig.VmName = "test-vm"
			// VddkLibDir is empty, so VDDK args should not be added

			mockCommandBuilder.EXPECT().AddArg("-i", "libvirt").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("-ic", appConfig.LibvirtUrl).Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("-ip", appConfig.SecretKey).Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("--hostname", appConfig.HostName).Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("--root", "first").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddPositional("--").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddPositional("test-vm").Return(mockCommandBuilder)

			err := conversion.addVirtV2vVsphereArgsForInspection(mockCommandBuilder)
			Expect(err).ToNot(HaveOccurred())
		})

		It("does NOT add conversion extra args (critical separation test)", func() {
			appConfig.LibvirtUrl = "vpx://user@vcenter.example.com/Datacenter/Cluster/esxi-host?no_verify=1"
			appConfig.SecretKey = "/etc/secret/secretKey"
			appConfig.HostName = "vcenter.example.com"
			appConfig.VmName = "test-vm"
			// Set ExtraArgs - these should NOT be applied for inspection
			appConfig.ExtraArgs = []string{"--parallel", "4", "--custom-flag"}

			mockCommandBuilder.EXPECT().AddArg("-i", "libvirt").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("-ic", appConfig.LibvirtUrl).Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("-ip", appConfig.SecretKey).Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("--hostname", appConfig.HostName).Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("--root", "first").Return(mockCommandBuilder)
			// Note: NO AddExtraArgs expectation - this is the critical test
			mockCommandBuilder.EXPECT().AddPositional("--").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddPositional("test-vm").Return(mockCommandBuilder)

			err := conversion.addVirtV2vVsphereArgsForInspection(mockCommandBuilder)
			Expect(err).ToNot(HaveOccurred())
		})

		It("adds vSphere args with custom root disk", func() {
			appConfig.LibvirtUrl = "vpx://user@vcenter.example.com/Datacenter/Cluster/esxi-host?no_verify=1"
			appConfig.SecretKey = "/etc/secret/secretKey"
			appConfig.HostName = "vcenter.example.com"
			appConfig.VmName = "test-vm"
			appConfig.RootDisk = "/dev/sdb"

			mockCommandBuilder.EXPECT().AddArg("-i", "libvirt").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("-ic", appConfig.LibvirtUrl).Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("-ip", appConfig.SecretKey).Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("--hostname", appConfig.HostName).Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("--root", "/dev/sdb").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddPositional("--").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddPositional("test-vm").Return(mockCommandBuilder)

			err := conversion.addVirtV2vVsphereArgsForInspection(mockCommandBuilder)
			Expect(err).ToNot(HaveOccurred())
		})

		It("adds vSphere args with static IPs", func() {
			appConfig.LibvirtUrl = "vpx://user@vcenter.example.com/Datacenter/Cluster/esxi-host?no_verify=1"
			appConfig.SecretKey = "/etc/secret/secretKey"
			appConfig.HostName = "vcenter.example.com"
			appConfig.VmName = "test-vm"
			appConfig.StaticIPs = "00:11:22:33:44:55:ip:192.168.1.100"

			mockCommandBuilder.EXPECT().AddArg("-i", "libvirt").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("-ic", appConfig.LibvirtUrl).Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("-ip", appConfig.SecretKey).Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("--hostname", appConfig.HostName).Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("--root", "first").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("--mac", "00:11:22:33:44:55:ip:192.168.1.100").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddPositional("--").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddPositional("test-vm").Return(mockCommandBuilder)

			err := conversion.addVirtV2vVsphereArgsForInspection(mockCommandBuilder)
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns error when addCommonArgs fails", func() {
			appConfig.LibvirtUrl = "vpx://user@vcenter.example.com/Datacenter/Cluster/esxi-host?no_verify=1"
			appConfig.SecretKey = "/etc/secret/secretKey"
			appConfig.HostName = "vcenter.example.com"
			appConfig.VmName = "test-vm"
			luksDir := "/etc/luks"
			appConfig.Luksdir = luksDir

			mockCommandBuilder.EXPECT().AddArg("-i", "libvirt").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("-ic", appConfig.LibvirtUrl).Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("-ip", appConfig.SecretKey).Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("--hostname", appConfig.HostName).Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("--root", "first").Return(mockCommandBuilder)

			mockFileSystem.EXPECT().Stat(luksDir).Return(nil, nil)
			mockFileSystem.EXPECT().ReadDir(luksDir).Return(nil, errors.New("permission denied"))

			err := conversion.addVirtV2vVsphereArgsForInspection(mockCommandBuilder)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("error adding LUKS keys"))
		})

		It("adds vSphere args with clevis for NBDE", func() {
			appConfig.LibvirtUrl = "vpx://user@vcenter.example.com/Datacenter/Cluster/esxi-host?no_verify=1"
			appConfig.SecretKey = "/etc/secret/secretKey"
			appConfig.HostName = "vcenter.example.com"
			appConfig.VmName = "test-vm"
			appConfig.NbdeClevis = true

			mockCommandBuilder.EXPECT().AddArg("-i", "libvirt").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("-ic", appConfig.LibvirtUrl).Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("-ip", appConfig.SecretKey).Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("--hostname", appConfig.HostName).Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("--root", "first").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArgs("--key", "all:clevis").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddPositional("--").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddPositional("test-vm").Return(mockCommandBuilder)

			err := conversion.addVirtV2vVsphereArgsForInspection(mockCommandBuilder)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
