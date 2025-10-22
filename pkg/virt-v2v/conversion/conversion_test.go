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

			mockCommandBuilder.EXPECT().AddArg("--root", "/dev/sda").Return(mockCommandBuilder)
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
