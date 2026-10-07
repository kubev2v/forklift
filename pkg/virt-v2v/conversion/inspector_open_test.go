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

	Describe("buildVirtInspectorRunArgs", func() {
		It("builds run argv with @@ placeholder and output redirect", func() {
			args, err := conversion.buildVirtInspectorRunArgs()
			Expect(err).ToNot(HaveOccurred())
			Expect(args).To(Equal([]string{
				"virt-inspector", "-v", "-x", "--format=raw",
				"@@", ">", config.InspectionOutputFile,
			}))
		})

		It("includes inspector extra args", func() {
			appConfig.InspectorExtraArgs = []string{"--no-applications", "--no-icon"}
			args, err := conversion.buildVirtInspectorRunArgs()
			Expect(err).ToNot(HaveOccurred())
			Expect(args).To(Equal([]string{
				"virt-inspector", "-v", "-x", "--format=raw",
				"--no-applications", "--no-icon",
				"@@", ">", config.InspectionOutputFile,
			}))
		})

		It("includes clevis key when NBDE is enabled", func() {
			appConfig.NbdeClevis = true
			args, err := conversion.buildVirtInspectorRunArgs()
			Expect(err).ToNot(HaveOccurred())
			Expect(args).To(ContainElements("--key", "all:clevis"))
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

			args, err := conversion.buildVirtInspectorRunArgs()
			Expect(err).ToNot(HaveOccurred())
			Expect(args).To(ContainElements(
				"--key", "all:file:/etc/luks/key1",
				"--key", "all:file:/etc/luks/key2",
			))
		})
	})
})
