package conversion

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/kubev2v/forklift/pkg/virt-v2v/config"
	"github.com/kubev2v/forklift/pkg/virt-v2v/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

var _ = Describe("Overlay", func() {
	var conversion *Conversion
	var mockCtrl *gomock.Controller
	var mockCommandExecutor *utils.MockCommandExecutor
	var mockCommandBuilder *utils.MockCommandBuilder
	var appConfig *config.AppConfig
	var workdir string

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockCommandExecutor = utils.NewMockCommandExecutor(mockCtrl)
		mockCommandBuilder = utils.NewMockCommandBuilder(mockCtrl)

		workdir = GinkgoT().TempDir()
		appConfig = &config.AppConfig{
			Workdir: workdir,
		}
		conversion = &Conversion{
			AppConfig:      appConfig,
			CommandBuilder: mockCommandBuilder,
		}
	})

	setupSingleDisk := func() string {
		linkPath := filepath.Join(workdir, "vm-sda")
		Expect(os.Symlink("/dev/block0", linkPath)).To(Succeed())
		conversion.Disks = []*Disk{
			{Path: "/dev/block0", Link: linkPath},
		}
		return linkPath
	}

	setupTwoDisks := func() (string, string) {
		linkA := filepath.Join(workdir, "vm-sda")
		linkB := filepath.Join(workdir, "vm-sdb")
		Expect(os.Symlink("/dev/block0", linkA)).To(Succeed())
		Expect(os.Symlink("/dev/block1", linkB)).To(Succeed())
		conversion.Disks = []*Disk{
			{Path: "/dev/block0", Link: linkA},
			{Path: "/dev/block1", Link: linkB},
		}
		return linkA, linkB
	}

	expectQemuImgCreate := func(diskPath, overlayPath string) {
		mockCommandBuilder.EXPECT().New("qemu-img").Return(mockCommandBuilder)
		mockCommandBuilder.EXPECT().AddPositional("create").Return(mockCommandBuilder)
		mockCommandBuilder.EXPECT().AddArg("-f", "qcow2").Return(mockCommandBuilder)
		mockCommandBuilder.EXPECT().AddArg("-b", diskPath).Return(mockCommandBuilder)
		mockCommandBuilder.EXPECT().AddArg("-F", "raw").Return(mockCommandBuilder)
		mockCommandBuilder.EXPECT().AddPositional(overlayPath).Return(mockCommandBuilder)
		mockCommandBuilder.EXPECT().Build().Return(mockCommandExecutor)
		mockCommandExecutor.EXPECT().SetStdout(os.Stdout)
		mockCommandExecutor.EXPECT().SetStderr(os.Stderr)
		mockCommandExecutor.EXPECT().Run().Return(nil)
	}

	expectQemuImgCommit := func(overlayPath string) {
		mockCommandBuilder.EXPECT().New("qemu-img").Return(mockCommandBuilder)
		mockCommandBuilder.EXPECT().AddPositional("commit").Return(mockCommandBuilder)
		mockCommandBuilder.EXPECT().AddPositional(overlayPath).Return(mockCommandBuilder)
		mockCommandBuilder.EXPECT().Build().Return(mockCommandExecutor)
		mockCommandExecutor.EXPECT().SetStdout(os.Stdout)
		mockCommandExecutor.EXPECT().SetStderr(os.Stderr)
		mockCommandExecutor.EXPECT().Run().Return(nil)
	}

	Describe("CreateOverlays", func() {
		It("creates overlay and rewires symlink for a single disk", func() {
			linkPath := setupSingleDisk()
			overlayPath := linkPath + ".qcow2"

			expectQemuImgCreate("/dev/block0", overlayPath)

			overlays, err := conversion.CreateOverlays()
			Expect(err).ToNot(HaveOccurred())
			Expect(overlays).To(HaveLen(1))
			Expect(overlays[0].Path).To(Equal(overlayPath))
			Expect(overlays[0].BackingPath).To(Equal("/dev/block0"))
			Expect(overlays[0].OriginalLink).To(Equal("/dev/block0"))

			target, err := os.Readlink(linkPath)
			Expect(err).ToNot(HaveOccurred())
			Expect(target).To(Equal(overlayPath))
		})

		It("creates overlays for multiple disks", func() {
			linkA, linkB := setupTwoDisks()

			expectQemuImgCreate("/dev/block0", linkA+".qcow2")
			expectQemuImgCreate("/dev/block1", linkB+".qcow2")

			overlays, err := conversion.CreateOverlays()
			Expect(err).ToNot(HaveOccurred())
			Expect(overlays).To(HaveLen(2))
		})

		It("cleans up earlier overlays when a later create fails", func() {
			linkA, linkB := setupTwoDisks()

			expectQemuImgCreate("/dev/block0", linkA+".qcow2")

			// Second disk: create fails
			mockCommandBuilder.EXPECT().New("qemu-img").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddPositional("create").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("-f", "qcow2").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("-b", "/dev/block1").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("-F", "raw").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddPositional(linkB + ".qcow2").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().Build().Return(mockCommandExecutor)
			mockCommandExecutor.EXPECT().SetStdout(os.Stdout)
			mockCommandExecutor.EXPECT().SetStderr(os.Stderr)
			mockCommandExecutor.EXPECT().Run().Return(errors.New("qemu-img failed"))

			_, err := conversion.CreateOverlays()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to create overlay for /dev/block1"))
		})

		It("handles empty disk list", func() {
			conversion.Disks = []*Disk{}

			overlays, err := conversion.CreateOverlays()
			Expect(err).ToNot(HaveOccurred())
			Expect(overlays).To(BeEmpty())
		})
	})

	Describe("CommitOverlays", func() {
		It("commits overlay and restores symlink", func() {
			linkPath := setupSingleDisk()
			overlayPath := linkPath + ".qcow2"

			// Create a dummy overlay file so Remove can find it
			Expect(os.WriteFile(overlayPath, []byte("fake"), 0644)).To(Succeed())

			// Rewire the symlink as CreateOverlays would
			Expect(os.Remove(linkPath)).To(Succeed())
			Expect(os.Symlink(overlayPath, linkPath)).To(Succeed())

			overlays := []*Overlay{{
				Path:         overlayPath,
				BackingPath:  "/dev/block0",
				Disk:         conversion.Disks[0],
				OriginalLink: "/dev/block0",
			}}

			expectQemuImgCommit(overlayPath)

			err := conversion.CommitOverlays(overlays)
			Expect(err).ToNot(HaveOccurred())

			// Overlay file should be removed
			_, err = os.Stat(overlayPath)
			Expect(os.IsNotExist(err)).To(BeTrue())

			// Symlink should be restored to original
			target, err := os.Readlink(linkPath)
			Expect(err).ToNot(HaveOccurred())
			Expect(target).To(Equal("/dev/block0"))
		})

		It("returns error when qemu-img commit fails", func() {
			linkPath := setupSingleDisk()
			overlayPath := linkPath + ".qcow2"

			overlays := []*Overlay{{
				Path:         overlayPath,
				BackingPath:  "/dev/block0",
				Disk:         conversion.Disks[0],
				OriginalLink: "/dev/block0",
			}}

			mockCommandBuilder.EXPECT().New("qemu-img").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddPositional("commit").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddPositional(overlayPath).Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().Build().Return(mockCommandExecutor)
			mockCommandExecutor.EXPECT().SetStdout(os.Stdout)
			mockCommandExecutor.EXPECT().SetStderr(os.Stderr)
			mockCommandExecutor.EXPECT().Run().Return(errors.New("commit failed"))

			err := conversion.CommitOverlays(overlays)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to commit overlay"))
		})
	})

	Describe("DiscardOverlays", func() {
		It("removes overlay files and restores symlinks", func() {
			linkPath := setupSingleDisk()
			overlayPath := linkPath + ".qcow2"

			Expect(os.WriteFile(overlayPath, []byte("fake"), 0644)).To(Succeed())

			// Rewire symlink as CreateOverlays would
			Expect(os.Remove(linkPath)).To(Succeed())
			Expect(os.Symlink(overlayPath, linkPath)).To(Succeed())

			overlays := []*Overlay{{
				Path:         overlayPath,
				BackingPath:  "/dev/block0",
				Disk:         conversion.Disks[0],
				OriginalLink: "/dev/block0",
			}}

			conversion.DiscardOverlays(overlays)

			// Overlay file should be removed
			_, err := os.Stat(overlayPath)
			Expect(os.IsNotExist(err)).To(BeTrue())

			// Symlink should be restored to original
			target, err := os.Readlink(linkPath)
			Expect(err).ToNot(HaveOccurred())
			Expect(target).To(Equal("/dev/block0"))
		})
	})

	Describe("RunInPlaceWithOverlay", func() {
		It("commits overlays on successful conversion", func() {
			linkPath := setupSingleDisk()
			overlayPath := linkPath + ".qcow2"

			expectQemuImgCreate("/dev/block0", overlayPath)
			expectQemuImgCommit(overlayPath)

			called := false
			err := conversion.RunInPlaceWithOverlay(func() error {
				called = true
				return nil
			})

			Expect(err).ToNot(HaveOccurred())
			Expect(called).To(BeTrue())
		})

		It("discards overlays on conversion failure", func() {
			linkPath := setupSingleDisk()
			overlayPath := linkPath + ".qcow2"

			expectQemuImgCreate("/dev/block0", overlayPath)

			convErr := errors.New("simulated virt-v2v failure")
			err := conversion.RunInPlaceWithOverlay(func() error {
				return convErr
			})

			Expect(err).To(Equal(convErr))

			// Verify symlink was restored to original target
			target, err := os.Readlink(linkPath)
			Expect(err).ToNot(HaveOccurred())
			Expect(target).To(Equal("/dev/block0"))
		})

		It("returns error when overlay setup fails", func() {
			linkPath := setupSingleDisk()
			overlayPath := linkPath + ".qcow2"

			// qemu-img create fails
			mockCommandBuilder.EXPECT().New("qemu-img").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddPositional("create").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("-f", "qcow2").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("-b", "/dev/block0").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddArg("-F", "raw").Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().AddPositional(overlayPath).Return(mockCommandBuilder)
			mockCommandBuilder.EXPECT().Build().Return(mockCommandExecutor)
			mockCommandExecutor.EXPECT().SetStdout(os.Stdout)
			mockCommandExecutor.EXPECT().SetStderr(os.Stderr)
			mockCommandExecutor.EXPECT().Run().Return(errors.New("no space"))

			called := false
			err := conversion.RunInPlaceWithOverlay(func() error {
				called = true
				return nil
			})

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("overlay setup failed"))
			Expect(called).To(BeFalse())
		})
	})
})
