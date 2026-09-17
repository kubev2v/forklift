package customize

import (
	"os"
	"path/filepath"

	"github.com/kubev2v/forklift/pkg/virt-v2v/config"
	"github.com/kubev2v/forklift/pkg/virt-v2v/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

var _ = Describe("Conversion args", func() {
	var customize *Customize
	var mockCtrl *gomock.Controller
	var mockFileSystem *utils.MockFileSystem
	var mockCommandBuilder *utils.MockCommandBuilder
	var mockEmbedTool *MockEmbedTool
	var appConfig *config.AppConfig

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockFileSystem = utils.NewMockFileSystem(mockCtrl)
		mockEmbedTool = NewMockEmbedTool(mockCtrl)
		mockCommandBuilder = utils.NewMockCommandBuilder(mockCtrl)

		appConfig = &config.AppConfig{
			Workdir: config.V2vOutputDir,
		}
		customize = &Customize{
			appConfig:          appConfig,
			commandBuilder:     mockCommandBuilder,
			fileSystem:         mockFileSystem,
			embeddedFileSystem: mockEmbedTool,
			operatingSystem:    utils.InspectionOS{Osinfo: "rhel9"},
		}
	})

	Describe("AppendConversionArgs", func() {
		It("appends linux run and firstboot scripts without --add disks", func() {
			runScripts := utils.ConvertMockDirEntryToOs([]utils.MockDirEntry{
				{FileName: "script1.sh", FileIsDir: false},
			})
			firstBootScripts := utils.ConvertMockDirEntryToOs([]utils.MockDirEntry{
				{FileName: "script1.sh", FileIsDir: false},
			})

			mockEmbedTool.EXPECT().CreateFilesFromFS(appConfig.Workdir).Return(nil)
			mockFileSystem.EXPECT().Stat(gomock.Any()).Return(nil, os.ErrNotExist)
			mockFileSystem.EXPECT().ReadDir(filepath.Join(config.V2vOutputDir, "scripts", "rhel", "run")).Return(runScripts, nil)
			mockFileSystem.EXPECT().ReadDir(filepath.Join(config.V2vOutputDir, "scripts", "rhel", "firstboot")).Return(firstBootScripts, nil)
			mockCommandBuilder.EXPECT().AddArg("--run", filepath.Join(config.V2vOutputDir, "scripts", "rhel", "run", "script1.sh")).Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("--firstboot", filepath.Join(config.V2vOutputDir, "scripts", "rhel", "firstboot", "script1.sh")).Return(mockCommandBuilder)

			err := customize.AppendConversionArgs(mockCommandBuilder)
			Expect(err).ToNot(HaveOccurred())
		})

		It("appends windows upload scripts without --add disks", func() {
			customize.operatingSystem = utils.InspectionOS{Osinfo: "win10"}
			winScripts := utils.ConvertMockDirEntryToOs([]utils.MockDirEntry{
				{FileName: "01_win_firstboot_setup.ps1", FileIsDir: false},
			})

			mockEmbedTool.EXPECT().CreateFilesFromFS(appConfig.Workdir).Return(nil)
			mockFileSystem.EXPECT().Stat(appConfig.DynamicScriptsDir).Return(nil, nil)
			mockFileSystem.EXPECT().ReadDir(appConfig.DynamicScriptsDir).Return(winScripts, nil)
			mockCommandBuilder.EXPECT().AddArg("--upload", gomock.Any()).Return(mockCommandBuilder).Times(2)
			mockCommandBuilder.EXPECT().AddArgs("--upload", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mockCommandBuilder)

			err := customize.AppendConversionArgs(mockCommandBuilder)
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns error when Prepare fails", func() {
			mockEmbedTool.EXPECT().CreateFilesFromFS(appConfig.Workdir).Return(os.ErrPermission)

			err := customize.AppendConversionArgs(mockCommandBuilder)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to create files from filesystem"))
		})
	})
})
