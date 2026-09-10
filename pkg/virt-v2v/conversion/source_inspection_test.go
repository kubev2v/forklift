package conversion

import (
	"os"

	"github.com/kubev2v/forklift/pkg/virt-v2v/config"
	"github.com/kubev2v/forklift/pkg/virt-v2v/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

var _ = Describe("Source inspection", func() {
	var conversion *Conversion
	var mockCtrl *gomock.Controller
	var mockCommandExecutor *utils.MockCommandExecutor
	var mockCommandBuilder *utils.MockCommandBuilder
	var appConfig *config.AppConfig

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockCommandExecutor = utils.NewMockCommandExecutor(mockCtrl)
		mockCommandBuilder = utils.NewMockCommandBuilder(mockCtrl)

		appConfig = &config.AppConfig{
			SupportsNoFstrim:     true,
			InspectionOutputFile: config.InspectionOutputFile,
			LibvirtUrl:           "vpx://user@vcenter.example.com/Datacenter/Cluster/esxi-host?no_verify=1",
			SecretKey:            "/etc/secret/secretKey",
			HostName:             "vcenter.example.com",
			VmName:               "test-vm",
		}
		conversion = &Conversion{
			AppConfig:      appConfig,
			CommandBuilder: mockCommandBuilder,
			fileSystem:     &utils.FileSystemImpl{},
		}
	})

	Describe("RunSourceV2vInspection", func() {
		It("invokes virt-v2v-open with virt-inspector against the vSphere source", func() {
			appConfig.InspectorExtraArgs = []string{"--inspector-extra"}
			runCmd := "virt-inspector -v -x --format=raw --inspector-extra @@ > \"/var/tmp/v2v/inspection.xml\""

			mockCommandBuilder.EXPECT().New("virt-v2v-open").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddFlag("-v").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddFlag("-x").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("--run", runCmd).Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("-i", "libvirt").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("-ic", appConfig.LibvirtUrl).Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("-ip", appConfig.SecretKey).Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddPositional("--").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddPositional("test-vm").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().Build().Return(mockCommandExecutor)
			mockCommandExecutor.EXPECT().SetStdout(os.Stdout)
			mockCommandExecutor.EXPECT().SetStderr(os.Stderr)
			mockCommandExecutor.EXPECT().Run().Return(nil)

			err := conversion.RunSourceV2vInspection()
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
