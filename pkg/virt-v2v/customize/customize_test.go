package customize

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/kubev2v/forklift/pkg/virt-v2v/config"
	"github.com/kubev2v/forklift/pkg/virt-v2v/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

func TestCustomize(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Customize test suite")
}

var _ = Describe("Customize", func() {
	var customize *Customize
	var mockCtrl *gomock.Controller
	var mockFileSystem *utils.MockFileSystem
	var mockCommandBuilder *utils.MockCommandBuilder
	var mockCommandExecutor *utils.MockCommandExecutor
	var mockEmbedTool *MockEmbedTool
	var appConfig *config.AppConfig
	var expectedFirstBootScripts []string
	var expectedRunScripts []string
	var disks []string
	var firstBootScripts []os.DirEntry
	var runScripts []os.DirEntry

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockFileSystem = utils.NewMockFileSystem(mockCtrl)
		mockEmbedTool = NewMockEmbedTool(mockCtrl)
		mockCommandBuilder = utils.NewMockCommandBuilder(mockCtrl)
		mockCommandExecutor = utils.NewMockCommandExecutor(mockCtrl)

		appConfig = &config.AppConfig{
			Workdir: config.V2vOutputDir,
		}
		customize = &Customize{
			appConfig:          appConfig,
			commandBuilder:     mockCommandBuilder,
			fileSystem:         mockFileSystem,
			embeddedFileSystem: mockEmbedTool,
		}

		expectedFirstBootScripts = []string{
			"/var/tmp/v2v/scripts/rhel/firstboot/script1.sh",
			"/var/tmp/v2v/scripts/rhel/firstboot/script2.sh",
		}
		expectedRunScripts = []string{
			"/var/tmp/v2v/scripts/rhel/run/script1.sh",
			"/var/tmp/v2v/scripts/rhel/run/script2.sh",
		}
		firstBootScripts = utils.ConvertMockDirEntryToOs([]utils.MockDirEntry{
			{FileName: "script1.sh", FileIsDir: false},
			{FileName: "script2.sh", FileIsDir: false},
			{FileName: "script1.ps1", FileIsDir: false},
			{FileName: "script2.ps1", FileIsDir: false},
			{FileName: "test-script1.sh", FileIsDir: false},
			{FileName: "test-script2.sh", FileIsDir: false},
			{FileName: "test-script1.ps1", FileIsDir: false},
			{FileName: "test-script2.ps1", FileIsDir: false},
			{FileName: "dir1", FileIsDir: true},
			{FileName: "dir2", FileIsDir: true},
		})
		runScripts = utils.ConvertMockDirEntryToOs([]utils.MockDirEntry{
			{FileName: "script1.sh", FileIsDir: false},
			{FileName: "script2.sh", FileIsDir: false},
			{FileName: "script1.ps1", FileIsDir: false},
			{FileName: "script2.ps1", FileIsDir: false},
			{FileName: "test-script1.sh", FileIsDir: false},
			{FileName: "test-script2.sh", FileIsDir: false},
			{FileName: "dir1", FileIsDir: true},
			{FileName: "dir2", FileIsDir: true},
		})
		disks = []string{
			"/var/tmp/v2v/new-vm-name-sda",
			"/var/tmp/v2v/new-vm-name-sdb",
		}
	})

	It("TestCustomizeRHELWithMock", func() {
		customize.disks = disks
		mockCommandBuilder.EXPECT().New("virt-customize").Return(mockCommandBuilder)
		for _, expectedScript := range expectedRunScripts {
			mockCommandBuilder.EXPECT().AddArg("--run", expectedScript).Return(mockCommandBuilder)
		}
		for _, expectedScript := range expectedFirstBootScripts {
			mockCommandBuilder.EXPECT().AddArg("--firstboot", expectedScript).Return(mockCommandBuilder)
		}
		for _, disk := range disks {
			mockCommandBuilder.EXPECT().AddArg("--add", disk).Return(mockCommandBuilder)
		}
		mockCommandBuilder.EXPECT().AddFlag("--verbose").Return(mockCommandBuilder)
		mockCommandBuilder.EXPECT().AddArg("--format", "raw").Return(mockCommandBuilder)
		mockCommandBuilder.EXPECT().Build().Return(mockCommandExecutor)
		mockCommandExecutor.EXPECT().Run().Return(nil)
		mockCommandExecutor.EXPECT().SetStdout(os.Stdout)
		mockCommandExecutor.EXPECT().SetStderr(os.Stderr)

		mockFileSystem.EXPECT().Stat(gomock.Any()).Return(nil, os.ErrNotExist)
		mockFileSystem.EXPECT().ReadDir(filepath.Join(config.V2vOutputDir, "scripts", "rhel", "run")).Return(runScripts, nil)
		mockFileSystem.EXPECT().ReadDir(filepath.Join(config.V2vOutputDir, "scripts", "rhel", "firstboot")).Return(firstBootScripts, nil)

		err := customize.customizeLinux()
		Expect(err).ToNot(HaveOccurred())
	})
	Describe("TestHandleStaticIPConfiguration", func() {
		It("StaticIPs - It passes", func() {
			appConfig.StaticIPs = "00:11:22:33:44:55:ip:192.168.1.100"
			expectedContent := "00:11:22:33:44:55:ip:192.168.1.100\n"
			expectedPath := filepath.Join(appConfig.Workdir, "macToIP")
			mockFileSystem.EXPECT().WriteFile(expectedPath, []byte(expectedContent), fs.FileMode(0755)).Return(nil)
			mockCommandBuilder.EXPECT().AddArg("--upload", fmt.Sprintf("%s:/tmp/macToIP", expectedPath)).Times(1)
			err := customize.handleStaticIPConfiguration(mockCommandBuilder)
			Expect(err).NotTo(HaveOccurred())
		})
		It("WriteFileFails - It fails", func() {
			appConfig.StaticIPs = "00:11:22:33:44:55:ip:192.168.1.100"
			expectedContent := "00:11:22:33:44:55:ip:192.168.1.100\n"
			expectedPath := filepath.Join(appConfig.Workdir, "macToIP")
			mockFileSystem.EXPECT().WriteFile(expectedPath, []byte(expectedContent), fs.FileMode(0755)).Return(errors.New("error"))
			mockCommandBuilder.EXPECT().AddArg("--upload", fmt.Sprintf("%s:/tmp/macToIP", expectedPath)).Times(0)
			err := customize.handleStaticIPConfiguration(mockCommandBuilder)
			Expect(err).To(HaveOccurred())
		})
	})
	Describe("TestAddFirstbootScripts", func() {
		It("Scripts - It passes", func() {
			mockFileSystem.EXPECT().ReadDir(filepath.Join(config.V2vOutputDir, "scripts", "rhel", "firstboot")).Return(firstBootScripts, nil)
			for _, expectedScript := range expectedFirstBootScripts {
				mockCommandBuilder.EXPECT().AddArg("--firstboot", expectedScript).Return(mockCommandBuilder)
			}
			err := customize.addRhelFirstbootScripts(mockCommandBuilder)
			Expect(err).ToNot(HaveOccurred())
		})
		It("NoScripts - It does not add", func() {
			mockFileSystem.EXPECT().ReadDir(filepath.Join(config.V2vOutputDir, "scripts", "rhel", "firstboot")).Return(nil, nil)
			err := customize.addRhelFirstbootScripts(mockCommandBuilder)
			Expect(err).ToNot(HaveOccurred())
		})
		It("ReadDirFails - It fails", func() {
			mockFileSystem.EXPECT().ReadDir(filepath.Join(config.V2vOutputDir, "scripts", "rhel", "firstboot")).Return(nil, errors.New("error"))
			err := customize.addRhelFirstbootScripts(mockCommandBuilder)
			Expect(err).To(HaveOccurred())
		})
	})
	Describe("TestAddRunScripts", func() {
		It("Scripts - It passes", func() {
			mockFileSystem.EXPECT().ReadDir(filepath.Join(config.V2vOutputDir, "scripts", "rhel", "run")).Return(runScripts, nil)
			for _, expectedScript := range expectedRunScripts {
				mockCommandBuilder.EXPECT().AddArg("--run", expectedScript).Return(mockCommandBuilder)
			}
			err := customize.addRhelRunScripts(mockCommandBuilder)
			Expect(err).ToNot(HaveOccurred())
		})
		It("NoScripts - It does not add", func() {
			mockFileSystem.EXPECT().ReadDir(filepath.Join(config.V2vOutputDir, "scripts", "rhel", "run")).Return(nil, nil)
			err := customize.addRhelRunScripts(mockCommandBuilder)
			Expect(err).ToNot(HaveOccurred())
		})
		It("ReadDirFails - It fails", func() {
			mockFileSystem.EXPECT().ReadDir(filepath.Join(config.V2vOutputDir, "scripts", "rhel", "run")).Return(nil, errors.New("error"))
			err := customize.addRhelRunScripts(mockCommandBuilder)
			Expect(err).To(HaveOccurred())
		})
	})
	It("TestAddDisksToCustomize", func() {
		customize.disks = disks
		for _, disk := range disks {
			mockCommandBuilder.EXPECT().AddArg("--add", disk).Return(mockCommandBuilder)
		}
		customize.addDisksToCustomize(mockCommandBuilder)
	})
})
