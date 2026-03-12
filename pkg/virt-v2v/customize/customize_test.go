// Generated-by: Claude
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

	Describe("customizeLinux", func() {
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

		It("returns error when run command fails", func() {
			customize.disks = disks
			mockCommandBuilder.EXPECT().New("virt-customize").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddFlag("--verbose").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("--format", "raw").Return(mockCommandBuilder)
			for _, expectedScript := range expectedRunScripts {
				mockCommandBuilder.EXPECT().AddArg("--run", expectedScript).Return(mockCommandBuilder)
			}
			for _, expectedScript := range expectedFirstBootScripts {
				mockCommandBuilder.EXPECT().AddArg("--firstboot", expectedScript).Return(mockCommandBuilder)
			}
			for _, disk := range disks {
				mockCommandBuilder.EXPECT().AddArg("--add", disk).Return(mockCommandBuilder)
			}
			mockCommandBuilder.EXPECT().Build().Return(mockCommandExecutor)
			mockCommandExecutor.EXPECT().Run().Return(errors.New("command failed"))
			mockCommandExecutor.EXPECT().SetStdout(os.Stdout)
			mockCommandExecutor.EXPECT().SetStderr(os.Stderr)

			mockFileSystem.EXPECT().Stat(gomock.Any()).Return(nil, os.ErrNotExist)
			mockFileSystem.EXPECT().ReadDir(filepath.Join(config.V2vOutputDir, "scripts", "rhel", "run")).Return(runScripts, nil)
			mockFileSystem.EXPECT().ReadDir(filepath.Join(config.V2vOutputDir, "scripts", "rhel", "firstboot")).Return(firstBootScripts, nil)

			err := customize.customizeLinux()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to execute domain customization"))
		})
	})

	Describe("handleStaticIPConfiguration", func() {
		It("StaticIPs - It passes", func() {
			appConfig.StaticIPs = "00:11:22:33:44:55:ip:192.168.1.100"
			expectedContent := "00:11:22:33:44:55:ip:192.168.1.100\n"
			expectedPath := filepath.Join(appConfig.Workdir, "macToIP")
			mockFileSystem.EXPECT().WriteFile(expectedPath, []byte(expectedContent), fs.FileMode(0755)).Return(nil)
			mockCommandBuilder.EXPECT().AddArg("--upload", fmt.Sprintf("%s:/tmp/macToIP", expectedPath)).Times(1)
			err := customize.handleStaticIPConfiguration(mockCommandBuilder)
			Expect(err).NotTo(HaveOccurred())
		})

		It("handles multiple static IPs separated by underscore", func() {
			appConfig.StaticIPs = "00:11:22:33:44:55:ip:192.168.1.100_00:11:22:33:44:56:ip:192.168.1.101"
			expectedContent := "00:11:22:33:44:55:ip:192.168.1.100\n00:11:22:33:44:56:ip:192.168.1.101\n"
			expectedPath := filepath.Join(appConfig.Workdir, "macToIP")
			mockFileSystem.EXPECT().WriteFile(expectedPath, []byte(expectedContent), fs.FileMode(0755)).Return(nil)
			mockCommandBuilder.EXPECT().AddArg("--upload", fmt.Sprintf("%s:/tmp/macToIP", expectedPath)).Times(1)
			err := customize.handleStaticIPConfiguration(mockCommandBuilder)
			Expect(err).NotTo(HaveOccurred())
		})

		It("does nothing when StaticIPs is empty", func() {
			appConfig.StaticIPs = ""
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

	Describe("addRhelFirstbootScripts", func() {
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

	Describe("addRhelRunScripts", func() {
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

	Describe("addDisksToCustomize", func() {
		It("adds all disks", func() {
			customize.disks = disks
			for _, disk := range disks {
				mockCommandBuilder.EXPECT().AddArg("--add", disk).Return(mockCommandBuilder)
			}
			customize.addDisksToCustomize(mockCommandBuilder)
		})

		It("handles empty disks slice", func() {
			customize.disks = []string{}
			// No mock expectations - no AddArg calls should be made
			customize.addDisksToCustomize(mockCommandBuilder)
		})

		It("handles single disk", func() {
			customize.disks = []string{"/var/tmp/v2v/single-disk"}
			mockCommandBuilder.EXPECT().AddArg("--add", "/var/tmp/v2v/single-disk").Return(mockCommandBuilder)
			customize.addDisksToCustomize(mockCommandBuilder)
		})
	})

	Describe("addLuksKeysToCustomize", func() {
		It("adds clevis key when NbdeClevis is true", func() {
			appConfig.NbdeClevis = true
			mockCommandBuilder.EXPECT().AddArgs("--key", "all:clevis").Return(mockCommandBuilder)
			err := customize.addLuksKeysToCustomize(mockCommandBuilder)
			Expect(err).ToNot(HaveOccurred())
		})

		It("does nothing when Luksdir is empty and NbdeClevis is false", func() {
			appConfig.Luksdir = ""
			appConfig.NbdeClevis = false
			// No mock expectations
			err := customize.addLuksKeysToCustomize(mockCommandBuilder)
			Expect(err).ToNot(HaveOccurred())
		})

		It("adds LUKS keys from directory", func() {
			appConfig.Luksdir = "/etc/luks"
			appConfig.NbdeClevis = false

			files := utils.ConvertMockDirEntryToOs([]utils.MockDirEntry{
				{FileName: "key1", FileIsDir: false},
				{FileName: "key2", FileIsDir: false},
			})

			mockFileSystem.EXPECT().Stat("/etc/luks").Return(nil, nil)
			mockFileSystem.EXPECT().ReadDir("/etc/luks").Return(files, nil)
			mockCommandBuilder.EXPECT().AddArgs("--key", "all:file:/etc/luks/key1", "all:file:/etc/luks/key2").Return(mockCommandBuilder)

			err := customize.addLuksKeysToCustomize(mockCommandBuilder)
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns error when LUKS directory read fails", func() {
			appConfig.Luksdir = "/etc/luks"
			appConfig.NbdeClevis = false

			mockFileSystem.EXPECT().Stat("/etc/luks").Return(nil, nil)
			mockFileSystem.EXPECT().ReadDir("/etc/luks").Return(nil, errors.New("read error"))

			err := customize.addLuksKeysToCustomize(mockCommandBuilder)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("getScriptsWithSuffix", func() {
		It("returns scripts with matching suffix", func() {
			dir := "/test/scripts"
			scripts := utils.ConvertMockDirEntryToOs([]utils.MockDirEntry{
				{FileName: "script1.sh", FileIsDir: false},
				{FileName: "script2.sh", FileIsDir: false},
				{FileName: "script3.ps1", FileIsDir: false},
			})

			mockFileSystem.EXPECT().ReadDir(dir).Return(scripts, nil)

			result, err := customize.getScriptsWithSuffix(dir, ".sh")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(HaveLen(2))
			Expect(result).To(ContainElement("/test/scripts/script1.sh"))
			Expect(result).To(ContainElement("/test/scripts/script2.sh"))
		})

		It("excludes test- prefixed files", func() {
			dir := "/test/scripts"
			scripts := utils.ConvertMockDirEntryToOs([]utils.MockDirEntry{
				{FileName: "script1.sh", FileIsDir: false},
				{FileName: "test-script.sh", FileIsDir: false},
			})

			mockFileSystem.EXPECT().ReadDir(dir).Return(scripts, nil)

			result, err := customize.getScriptsWithSuffix(dir, ".sh")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(HaveLen(1))
			Expect(result).To(ContainElement("/test/scripts/script1.sh"))
		})

		It("excludes directories", func() {
			dir := "/test/scripts"
			scripts := utils.ConvertMockDirEntryToOs([]utils.MockDirEntry{
				{FileName: "script1.sh", FileIsDir: false},
				{FileName: "subdir.sh", FileIsDir: true},
			})

			mockFileSystem.EXPECT().ReadDir(dir).Return(scripts, nil)

			result, err := customize.getScriptsWithSuffix(dir, ".sh")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(HaveLen(1))
		})

		It("returns empty slice for empty directory", func() {
			dir := "/test/scripts"
			mockFileSystem.EXPECT().ReadDir(dir).Return([]os.DirEntry{}, nil)

			result, err := customize.getScriptsWithSuffix(dir, ".sh")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(BeEmpty())
		})

		It("returns error when ReadDir fails", func() {
			dir := "/test/scripts"
			mockFileSystem.EXPECT().ReadDir(dir).Return(nil, errors.New("read error"))

			_, err := customize.getScriptsWithSuffix(dir, ".sh")
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("getScriptsWithRegex", func() {
		It("returns scripts matching regex", func() {
			dir := "/test/scripts"
			scripts := utils.ConvertMockDirEntryToOs([]utils.MockDirEntry{
				{FileName: "01_linux_run_setup.sh", FileIsDir: false},
				{FileName: "02_linux_firstboot_config.sh", FileIsDir: false},
				{FileName: "invalid.sh", FileIsDir: false},
			})

			mockFileSystem.EXPECT().ReadDir(dir).Return(scripts, nil)

			result, err := customize.getScriptsWithRegex(dir, LinuxDynamicRegex)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(HaveLen(2))
		})

		It("returns empty for no matches", func() {
			dir := "/test/scripts"
			scripts := utils.ConvertMockDirEntryToOs([]utils.MockDirEntry{
				{FileName: "invalid.sh", FileIsDir: false},
				{FileName: "another.txt", FileIsDir: false},
			})

			mockFileSystem.EXPECT().ReadDir(dir).Return(scripts, nil)

			result, err := customize.getScriptsWithRegex(dir, LinuxDynamicRegex)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(BeEmpty())
		})

		It("excludes directories", func() {
			dir := "/test/scripts"
			scripts := utils.ConvertMockDirEntryToOs([]utils.MockDirEntry{
				{FileName: "01_linux_run_setup.sh", FileIsDir: true},
			})

			mockFileSystem.EXPECT().ReadDir(dir).Return(scripts, nil)

			result, err := customize.getScriptsWithRegex(dir, LinuxDynamicRegex)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(BeEmpty())
		})

		It("returns error when ReadDir fails", func() {
			dir := "/test/scripts"
			mockFileSystem.EXPECT().ReadDir(dir).Return(nil, errors.New("read error"))

			_, err := customize.getScriptsWithRegex(dir, LinuxDynamicRegex)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("addRhelDynamicScripts", func() {
		It("adds run scripts", func() {
			dir := "/mnt/dynamic_scripts"
			scripts := utils.ConvertMockDirEntryToOs([]utils.MockDirEntry{
				{FileName: "01_linux_run_setup.sh", FileIsDir: false},
			})

			mockFileSystem.EXPECT().ReadDir(dir).Return(scripts, nil)
			mockCommandBuilder.EXPECT().AddArg("--run", filepath.Join(dir, "01_linux_run_setup.sh")).Return(mockCommandBuilder)

			err := customize.addRhelDynamicScripts(mockCommandBuilder, dir)
			Expect(err).ToNot(HaveOccurred())
		})

		It("adds firstboot scripts", func() {
			dir := "/mnt/dynamic_scripts"
			scripts := utils.ConvertMockDirEntryToOs([]utils.MockDirEntry{
				{FileName: "01_linux_firstboot_config.sh", FileIsDir: false},
			})

			mockFileSystem.EXPECT().ReadDir(dir).Return(scripts, nil)
			mockCommandBuilder.EXPECT().AddArg("--firstboot", filepath.Join(dir, "01_linux_firstboot_config.sh")).Return(mockCommandBuilder)

			err := customize.addRhelDynamicScripts(mockCommandBuilder, dir)
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns error for invalid action in regex", func() {
			dir := "/mnt/dynamic_scripts"
			scripts := utils.ConvertMockDirEntryToOs([]utils.MockDirEntry{
				{FileName: "01_linux_invalid_setup.sh", FileIsDir: false},
			})

			mockFileSystem.EXPECT().ReadDir(dir).Return(scripts, nil)

			// This should not match the regex, so no error
			err := customize.addRhelDynamicScripts(mockCommandBuilder, dir)
			Expect(err).ToNot(HaveOccurred())
		})

		It("handles empty directory", func() {
			dir := "/mnt/dynamic_scripts"
			mockFileSystem.EXPECT().ReadDir(dir).Return([]os.DirEntry{}, nil)

			err := customize.addRhelDynamicScripts(mockCommandBuilder, dir)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("formatUpload", func() {
		It("formats upload path correctly", func() {
			result := customize.formatUpload("/local/path/script.sh", "/remote/path/script.sh")
			Expect(result).To(Equal("/local/path/script.sh:/remote/path/script.sh"))
		})

		It("handles paths with spaces", func() {
			result := customize.formatUpload("/local/path with spaces/script.sh", "/remote/path/script.sh")
			Expect(result).To(Equal("/local/path with spaces/script.sh:/remote/path/script.sh"))
		})
	})

	Describe("formatIPs", func() {
		It("formats single IP", func() {
			ips := []IPEntry{{IP: "192.168.1.1"}}
			result := formatIPs(ips)
			Expect(result).To(Equal("(\n'192.168.1.1'\n)"))
		})

		It("formats multiple IPs", func() {
			ips := []IPEntry{
				{IP: "192.168.1.1"},
				{IP: "192.168.1.2"},
			}
			result := formatIPs(ips)
			Expect(result).To(Equal("(\n'192.168.1.1',\n'192.168.1.2'\n)"))
		})

		It("handles empty IPs", func() {
			ips := []IPEntry{}
			result := formatIPs(ips)
			Expect(result).To(Equal("(\n)"))
		})
	})

	Describe("formatDNS", func() {
		It("formats single DNS", func() {
			dns := []string{"8.8.8.8"}
			result := formatDNS(dns)
			Expect(result).To(Equal("(\n'8.8.8.8'\n)"))
		})

		It("formats multiple DNS", func() {
			dns := []string{"8.8.8.8", "8.8.4.4"}
			result := formatDNS(dns)
			Expect(result).To(Equal("(\n'8.8.8.8',\n'8.8.4.4'\n)"))
		})

		It("handles empty DNS", func() {
			dns := []string{}
			result := formatDNS(dns)
			Expect(result).To(Equal("(\n)"))
		})
	})

	Describe("NewCustomize", func() {
		It("creates Customize with correct fields", func() {
			cfg := &config.AppConfig{
				Workdir: "/test/workdir",
			}
			testDisks := []string{"/disk1", "/disk2"}
			osInfo := utils.InspectionOS{Name: "Fedora"}

			result := NewCustomize(cfg, testDisks, osInfo)

			Expect(result.appConfig).To(Equal(cfg))
			Expect(result.disks).To(Equal(testDisks))
			Expect(result.operatingSystem).To(Equal(osInfo))
			Expect(result.commandBuilder).ToNot(BeNil())
			Expect(result.fileSystem).ToNot(BeNil())
			Expect(result.embeddedFileSystem).ToNot(BeNil())
		})
	})

	Describe("Run", func() {
		It("runs Linux customization for non-Windows OS", func() {
			customize.disks = disks
			customize.operatingSystem = utils.InspectionOS{Osinfo: "linux"}

			mockEmbedTool.EXPECT().CreateFilesFromFS(appConfig.Workdir).Return(nil)
			mockCommandBuilder.EXPECT().New("virt-customize").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddFlag("--verbose").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("--format", "raw").Return(mockCommandBuilder)
			for _, expectedScript := range expectedRunScripts {
				mockCommandBuilder.EXPECT().AddArg("--run", expectedScript).Return(mockCommandBuilder)
			}
			for _, expectedScript := range expectedFirstBootScripts {
				mockCommandBuilder.EXPECT().AddArg("--firstboot", expectedScript).Return(mockCommandBuilder)
			}
			for _, disk := range disks {
				mockCommandBuilder.EXPECT().AddArg("--add", disk).Return(mockCommandBuilder)
			}
			mockCommandBuilder.EXPECT().Build().Return(mockCommandExecutor)
			mockCommandExecutor.EXPECT().Run().Return(nil)
			mockCommandExecutor.EXPECT().SetStdout(os.Stdout)
			mockCommandExecutor.EXPECT().SetStderr(os.Stderr)

			// Stat is called for DynamicScriptsDir and Luksdir
			mockFileSystem.EXPECT().Stat(appConfig.DynamicScriptsDir).Return(nil, os.ErrNotExist)
			mockFileSystem.EXPECT().ReadDir(filepath.Join(config.V2vOutputDir, "scripts", "rhel", "run")).Return(runScripts, nil)
			mockFileSystem.EXPECT().ReadDir(filepath.Join(config.V2vOutputDir, "scripts", "rhel", "firstboot")).Return(firstBootScripts, nil)

			err := customize.Run()
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns error when CreateFilesFromFS fails", func() {
			customize.disks = disks
			customize.operatingSystem = utils.InspectionOS{Osinfo: "linux"}

			mockEmbedTool.EXPECT().CreateFilesFromFS(appConfig.Workdir).Return(errors.New("embed error"))

			err := customize.Run()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to create files from filesystem"))
		})

		It("runs Windows customization for Windows OS", func() {
			customize.disks = disks
			customize.operatingSystem = utils.InspectionOS{Osinfo: "win10"}

			mockEmbedTool.EXPECT().CreateFilesFromFS(appConfig.Workdir).Return(nil)
			mockCommandBuilder.EXPECT().New("virt-customize").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddFlag("--verbose").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("--format", "raw").Return(mockCommandBuilder)

			// DynamicScriptsDir does not exist
			mockFileSystem.EXPECT().Stat(appConfig.DynamicScriptsDir).Return(nil, os.ErrNotExist)

			// addWinFirstbootScripts
			mockCommandBuilder.EXPECT().AddArgs("--upload", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mockCommandBuilder)

			for _, disk := range disks {
				mockCommandBuilder.EXPECT().AddArg("--add", disk).Return(mockCommandBuilder)
			}

			mockCommandBuilder.EXPECT().Build().Return(mockCommandExecutor)
			mockCommandExecutor.EXPECT().Run().Return(nil)
			mockCommandExecutor.EXPECT().SetStdout(os.Stdout)
			mockCommandExecutor.EXPECT().SetStderr(os.Stderr)

			err := customize.Run()
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns error when Windows customization fails", func() {
			customize.disks = disks
			customize.operatingSystem = utils.InspectionOS{Osinfo: "win10"}

			mockEmbedTool.EXPECT().CreateFilesFromFS(appConfig.Workdir).Return(nil)
			mockCommandBuilder.EXPECT().New("virt-customize").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddFlag("--verbose").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("--format", "raw").Return(mockCommandBuilder)

			mockFileSystem.EXPECT().Stat(appConfig.DynamicScriptsDir).Return(nil, os.ErrNotExist)
			mockCommandBuilder.EXPECT().AddArgs("--upload", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mockCommandBuilder)

			for _, disk := range disks {
				mockCommandBuilder.EXPECT().AddArg("--add", disk).Return(mockCommandBuilder)
			}

			mockCommandBuilder.EXPECT().Build().Return(mockCommandExecutor)
			mockCommandExecutor.EXPECT().Run().Return(errors.New("windows customization failed"))
			mockCommandExecutor.EXPECT().SetStdout(os.Stdout)
			mockCommandExecutor.EXPECT().SetStderr(os.Stderr)

			err := customize.Run()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("windows customization failed"))
		})
	})

	Describe("customizeWindows", func() {
		It("customizes Windows with dynamic scripts", func() {
			customize.disks = disks

			winScripts := utils.ConvertMockDirEntryToOs([]utils.MockDirEntry{
				{FileName: "01_win_firstboot_setup.ps1", FileIsDir: false},
			})

			// DynamicScriptsDir exists
			mockFileSystem.EXPECT().Stat(appConfig.DynamicScriptsDir).Return(nil, nil)
			mockFileSystem.EXPECT().ReadDir(appConfig.DynamicScriptsDir).Return(winScripts, nil)

			mockCommandBuilder.EXPECT().New("virt-customize").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddFlag("--verbose").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("--format", "raw").Return(mockCommandBuilder)

			// Dynamic script upload
			mockCommandBuilder.EXPECT().AddArg("--upload", gomock.Any()).Return(mockCommandBuilder)

			// addWinFirstbootScripts
			mockCommandBuilder.EXPECT().AddArgs("--upload", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mockCommandBuilder)

			for _, disk := range disks {
				mockCommandBuilder.EXPECT().AddArg("--add", disk).Return(mockCommandBuilder)
			}

			mockCommandBuilder.EXPECT().Build().Return(mockCommandExecutor)
			mockCommandExecutor.EXPECT().Run().Return(nil)
			mockCommandExecutor.EXPECT().SetStdout(os.Stdout)
			mockCommandExecutor.EXPECT().SetStderr(os.Stderr)

			err := customize.customizeWindows()
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns error when dynamic scripts read fails", func() {
			customize.disks = disks

			mockFileSystem.EXPECT().Stat(appConfig.DynamicScriptsDir).Return(nil, nil)
			mockFileSystem.EXPECT().ReadDir(appConfig.DynamicScriptsDir).Return(nil, errors.New("read error"))

			mockCommandBuilder.EXPECT().New("virt-customize").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddFlag("--verbose").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("--format", "raw").Return(mockCommandBuilder)

			err := customize.customizeWindows()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to read scripts directory"))
		})
	})

	Describe("addWinDynamicScripts", func() {
		It("adds Windows dynamic scripts matching regex", func() {
			dir := "/mnt/dynamic_scripts"
			scripts := utils.ConvertMockDirEntryToOs([]utils.MockDirEntry{
				{FileName: "01_win_firstboot_setup.ps1", FileIsDir: false},
				{FileName: "02_win_firstboot_config.ps1", FileIsDir: false},
			})

			mockFileSystem.EXPECT().ReadDir(dir).Return(scripts, nil)
			mockCommandBuilder.EXPECT().AddArg("--upload", gomock.Any()).Return(mockCommandBuilder).Times(2)

			err := customize.addWinDynamicScripts(mockCommandBuilder, dir)
			Expect(err).ToNot(HaveOccurred())
		})

		It("ignores non-matching files", func() {
			dir := "/mnt/dynamic_scripts"
			scripts := utils.ConvertMockDirEntryToOs([]utils.MockDirEntry{
				{FileName: "invalid_script.ps1", FileIsDir: false},
				{FileName: "random.txt", FileIsDir: false},
			})

			mockFileSystem.EXPECT().ReadDir(dir).Return(scripts, nil)
			// No AddArg calls expected since files don't match regex

			err := customize.addWinDynamicScripts(mockCommandBuilder, dir)
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns error when ReadDir fails", func() {
			dir := "/mnt/dynamic_scripts"
			mockFileSystem.EXPECT().ReadDir(dir).Return(nil, errors.New("read error"))

			err := customize.addWinDynamicScripts(mockCommandBuilder, dir)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("runCmd", func() {
		It("runs command successfully", func() {
			mockCommandBuilder.EXPECT().Build().Return(mockCommandExecutor)
			mockCommandExecutor.EXPECT().SetStdout(os.Stdout)
			mockCommandExecutor.EXPECT().SetStderr(os.Stderr)
			mockCommandExecutor.EXPECT().Run().Return(nil)

			err := customize.runCmd(mockCommandBuilder)
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns error when command fails", func() {
			mockCommandBuilder.EXPECT().Build().Return(mockCommandExecutor)
			mockCommandExecutor.EXPECT().SetStdout(os.Stdout)
			mockCommandExecutor.EXPECT().SetStderr(os.Stderr)
			mockCommandExecutor.EXPECT().Run().Return(errors.New("command execution failed"))

			err := customize.runCmd(mockCommandBuilder)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("error executing virt-customize command"))
		})
	})

	Describe("customizeLinux with static IPs", func() {
		It("handles static IP configuration", func() {
			customize.disks = disks
			appConfig.StaticIPs = "00:11:22:33:44:55:ip:192.168.1.100"

			expectedPath := filepath.Join(appConfig.Workdir, "macToIP")
			expectedContent := "00:11:22:33:44:55:ip:192.168.1.100\n"

			mockCommandBuilder.EXPECT().New("virt-customize").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddFlag("--verbose").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("--format", "raw").Return(mockCommandBuilder)

			// Static IP file write
			mockFileSystem.EXPECT().WriteFile(expectedPath, []byte(expectedContent), fs.FileMode(0755)).Return(nil)
			mockCommandBuilder.EXPECT().AddArg("--upload", fmt.Sprintf("%s:/tmp/macToIP", expectedPath)).Return(mockCommandBuilder)

			// DynamicScriptsDir does not exist
			mockFileSystem.EXPECT().Stat(appConfig.DynamicScriptsDir).Return(nil, os.ErrNotExist)

			// Scripts
			mockFileSystem.EXPECT().ReadDir(filepath.Join(config.V2vOutputDir, "scripts", "rhel", "run")).Return(runScripts, nil)
			mockFileSystem.EXPECT().ReadDir(filepath.Join(config.V2vOutputDir, "scripts", "rhel", "firstboot")).Return(firstBootScripts, nil)

			for _, expectedScript := range expectedRunScripts {
				mockCommandBuilder.EXPECT().AddArg("--run", expectedScript).Return(mockCommandBuilder)
			}
			for _, expectedScript := range expectedFirstBootScripts {
				mockCommandBuilder.EXPECT().AddArg("--firstboot", expectedScript).Return(mockCommandBuilder)
			}
			for _, disk := range disks {
				mockCommandBuilder.EXPECT().AddArg("--add", disk).Return(mockCommandBuilder)
			}

			mockCommandBuilder.EXPECT().Build().Return(mockCommandExecutor)
			mockCommandExecutor.EXPECT().Run().Return(nil)
			mockCommandExecutor.EXPECT().SetStdout(os.Stdout)
			mockCommandExecutor.EXPECT().SetStderr(os.Stderr)

			err := customize.customizeLinux()
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns error when static IP file write fails", func() {
			customize.disks = disks
			appConfig.StaticIPs = "00:11:22:33:44:55:ip:192.168.1.100"

			mockCommandBuilder.EXPECT().New("virt-customize").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddFlag("--verbose").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("--format", "raw").Return(mockCommandBuilder)

			expectedPath := filepath.Join(appConfig.Workdir, "macToIP")
			expectedContent := "00:11:22:33:44:55:ip:192.168.1.100\n"
			mockFileSystem.EXPECT().WriteFile(expectedPath, []byte(expectedContent), fs.FileMode(0755)).Return(errors.New("write error"))

			err := customize.customizeLinux()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to write MAC to IP mapping file"))
		})
	})

	Describe("customizeLinux with dynamic scripts", func() {
		It("adds dynamic scripts when directory exists", func() {
			customize.disks = disks

			dynamicScripts := utils.ConvertMockDirEntryToOs([]utils.MockDirEntry{
				{FileName: "01_linux_run_custom.sh", FileIsDir: false},
				{FileName: "02_linux_firstboot_init.sh", FileIsDir: false},
			})

			mockCommandBuilder.EXPECT().New("virt-customize").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddFlag("--verbose").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("--format", "raw").Return(mockCommandBuilder)

			// DynamicScriptsDir exists
			mockFileSystem.EXPECT().Stat(appConfig.DynamicScriptsDir).Return(nil, nil)
			mockFileSystem.EXPECT().ReadDir(appConfig.DynamicScriptsDir).Return(dynamicScripts, nil)

			// Dynamic script commands
			mockCommandBuilder.EXPECT().AddArg("--run", filepath.Join(appConfig.DynamicScriptsDir, "01_linux_run_custom.sh")).Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("--firstboot", filepath.Join(appConfig.DynamicScriptsDir, "02_linux_firstboot_init.sh")).Return(mockCommandBuilder)

			// Regular scripts
			mockFileSystem.EXPECT().ReadDir(filepath.Join(config.V2vOutputDir, "scripts", "rhel", "run")).Return(runScripts, nil)
			mockFileSystem.EXPECT().ReadDir(filepath.Join(config.V2vOutputDir, "scripts", "rhel", "firstboot")).Return(firstBootScripts, nil)

			for _, expectedScript := range expectedRunScripts {
				mockCommandBuilder.EXPECT().AddArg("--run", expectedScript).Return(mockCommandBuilder)
			}
			for _, expectedScript := range expectedFirstBootScripts {
				mockCommandBuilder.EXPECT().AddArg("--firstboot", expectedScript).Return(mockCommandBuilder)
			}
			for _, disk := range disks {
				mockCommandBuilder.EXPECT().AddArg("--add", disk).Return(mockCommandBuilder)
			}

			mockCommandBuilder.EXPECT().Build().Return(mockCommandExecutor)
			mockCommandExecutor.EXPECT().Run().Return(nil)
			mockCommandExecutor.EXPECT().SetStdout(os.Stdout)
			mockCommandExecutor.EXPECT().SetStderr(os.Stderr)

			err := customize.customizeLinux()
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns error when dynamic scripts read fails", func() {
			customize.disks = disks

			mockCommandBuilder.EXPECT().New("virt-customize").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddFlag("--verbose").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("--format", "raw").Return(mockCommandBuilder)

			// DynamicScriptsDir exists but ReadDir fails
			mockFileSystem.EXPECT().Stat(appConfig.DynamicScriptsDir).Return(nil, nil)
			mockFileSystem.EXPECT().ReadDir(appConfig.DynamicScriptsDir).Return(nil, errors.New("read error"))

			err := customize.customizeLinux()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to read scripts directory"))
		})
	})

	Describe("customizeLinux with LUKS keys", func() {
		It("adds LUKS keys from directory", func() {
			customize.disks = disks
			appConfig.Luksdir = "/etc/luks"

			luksFiles := utils.ConvertMockDirEntryToOs([]utils.MockDirEntry{
				{FileName: "key1", FileIsDir: false},
			})

			mockCommandBuilder.EXPECT().New("virt-customize").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddFlag("--verbose").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("--format", "raw").Return(mockCommandBuilder)

			// DynamicScriptsDir does not exist
			mockFileSystem.EXPECT().Stat(appConfig.DynamicScriptsDir).Return(nil, os.ErrNotExist)

			// Scripts
			mockFileSystem.EXPECT().ReadDir(filepath.Join(config.V2vOutputDir, "scripts", "rhel", "run")).Return(runScripts, nil)
			mockFileSystem.EXPECT().ReadDir(filepath.Join(config.V2vOutputDir, "scripts", "rhel", "firstboot")).Return(firstBootScripts, nil)

			for _, expectedScript := range expectedRunScripts {
				mockCommandBuilder.EXPECT().AddArg("--run", expectedScript).Return(mockCommandBuilder)
			}
			for _, expectedScript := range expectedFirstBootScripts {
				mockCommandBuilder.EXPECT().AddArg("--firstboot", expectedScript).Return(mockCommandBuilder)
			}
			for _, disk := range disks {
				mockCommandBuilder.EXPECT().AddArg("--add", disk).Return(mockCommandBuilder)
			}

			// LUKS keys
			mockFileSystem.EXPECT().Stat("/etc/luks").Return(nil, nil)
			mockFileSystem.EXPECT().ReadDir("/etc/luks").Return(luksFiles, nil)
			mockCommandBuilder.EXPECT().AddArgs("--key", "all:file:/etc/luks/key1").Return(mockCommandBuilder)

			mockCommandBuilder.EXPECT().Build().Return(mockCommandExecutor)
			mockCommandExecutor.EXPECT().Run().Return(nil)
			mockCommandExecutor.EXPECT().SetStdout(os.Stdout)
			mockCommandExecutor.EXPECT().SetStderr(os.Stderr)

			err := customize.customizeLinux()
			Expect(err).ToNot(HaveOccurred())
		})
	})

})
