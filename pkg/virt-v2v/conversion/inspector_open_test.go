package conversion

import (
	"github.com/kubev2v/forklift/pkg/virt-v2v/config"
	"github.com/kubev2v/forklift/pkg/virt-v2v/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

var _ = Describe("inspector_open", func() {
	var conversion *Conversion
	var mockFileSystem *utils.MockFileSystem
	var appConfig *config.AppConfig

	BeforeEach(func() {
		mockFileSystem = utils.NewMockFileSystem(gomock.NewController(GinkgoT()))
		appConfig = &config.AppConfig{
			InspectionOutputFile: config.InspectionOutputFile,
		}
		conversion = &Conversion{
			AppConfig:  appConfig,
			fileSystem: mockFileSystem,
		}
	})

	Describe("buildVirtInspectorRunCommand", func() {
		It("builds a run command with @@ placeholder and output redirect", func() {
			cmd, err := conversion.buildVirtInspectorRunCommand()
			Expect(err).ToNot(HaveOccurred())
			Expect(cmd).To(Equal("virt-inspector -v -x --format=raw @@ > \"/var/tmp/v2v/inspection.xml\""))
		})

		It("includes inspector extra args", func() {
			appConfig.InspectorExtraArgs = []string{"--no-applications", "--no-icon"}
			cmd, err := conversion.buildVirtInspectorRunCommand()
			Expect(err).ToNot(HaveOccurred())
			Expect(cmd).To(ContainSubstring("--no-applications --no-icon @@"))
		})

		It("includes clevis key when NBDE is enabled", func() {
			appConfig.NbdeClevis = true
			cmd, err := conversion.buildVirtInspectorRunCommand()
			Expect(err).ToNot(HaveOccurred())
			Expect(cmd).To(ContainSubstring("--key all:clevis"))
		})

		It("includes LUKS key files from the LUKS directory", func() {
			luksDir := "/etc/luks"
			appConfig.Luksdir = luksDir
			mockFileSystem.EXPECT().Stat(luksDir).Return(nil, nil)
			luksFiles := utils.ConvertMockDirEntryToOs([]utils.MockDirEntry{
				{FileName: "key1", FileIsDir: false},
				{FileName: "key2", FileIsDir: false},
			})
			mockFileSystem.EXPECT().ReadDir(luksDir).Return(luksFiles, nil)

			cmd, err := conversion.buildVirtInspectorRunCommand()
			Expect(err).ToNot(HaveOccurred())
			Expect(cmd).To(ContainSubstring("--key all:file:/etc/luks/key1"))
			Expect(cmd).To(ContainSubstring("--key all:file:/etc/luks/key2"))
		})
	})
})
