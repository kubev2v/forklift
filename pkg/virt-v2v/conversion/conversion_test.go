package conversion

import (
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

	It("passes virt-v2v-inspection",
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
	It("passes virt-v2v-inspection",
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
	It("adds common args with root disk and static IPs",
		func() {
			appConfig := config.AppConfig{
				RootDisk:  "/dev/sda",
				StaticIPs: "00:11:22:33:44:55:ip:192.168.1.100_00:11:22:33:44:56:ip:192.168.1.101",
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
			mockCommandBuilder.EXPECT().AddArg("--mac", "00:11:22:33:44:56:ip:192.168.1.101").Return(mockCommandBuilder)

			err := conversion.addCommonArgs(mockCommandBuilder)
			Expect(err).ToNot(HaveOccurred())
		},
	)
	It("adds common args with root disk and static IPs",
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
})
