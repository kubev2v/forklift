// Generated-by: Claude
package utils

import (
	"errors"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

func TestUtils(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Utils test suite")
}

var _ = Describe("Utils", func() {
	var mockCtrl *gomock.Controller
	var mockFileSystem *MockFileSystem
	var mockCommandBuilder *MockCommandBuilder

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockFileSystem = NewMockFileSystem(mockCtrl)
		mockCommandBuilder = NewMockCommandBuilder(mockCtrl)
	})

	Describe("AddLUKSKeys", func() {
		It("adds LUKS keys when directory exists and has files", func() {
			luksDir := "/etc/luks"
			files := ConvertMockDirEntryToOs([]MockDirEntry{
				{FileName: "key1", FileIsDir: false},
				{FileName: "key2", FileIsDir: false},
			})

			mockFileSystem.EXPECT().Stat(luksDir).Return(nil, nil)
			mockFileSystem.EXPECT().ReadDir(luksDir).Return(files, nil)
			mockCommandBuilder.EXPECT().AddArgs("--key", "all:file:/etc/luks/key1", "all:file:/etc/luks/key2").Return(mockCommandBuilder)

			err := AddLUKSKeys(mockFileSystem, mockCommandBuilder, luksDir)
			Expect(err).ToNot(HaveOccurred())
		})

		It("skips directories in LUKS directory", func() {
			luksDir := "/etc/luks"
			files := ConvertMockDirEntryToOs([]MockDirEntry{
				{FileName: "key1", FileIsDir: false},
				{FileName: "subdir", FileIsDir: true},
			})

			mockFileSystem.EXPECT().Stat(luksDir).Return(nil, nil)
			mockFileSystem.EXPECT().ReadDir(luksDir).Return(files, nil)
			mockCommandBuilder.EXPECT().AddArgs("--key", "all:file:/etc/luks/key1").Return(mockCommandBuilder)

			err := AddLUKSKeys(mockFileSystem, mockCommandBuilder, luksDir)
			Expect(err).ToNot(HaveOccurred())
		})

		It("skips files starting with ..", func() {
			luksDir := "/etc/luks"
			files := ConvertMockDirEntryToOs([]MockDirEntry{
				{FileName: "key1", FileIsDir: false},
				{FileName: "..hidden", FileIsDir: false},
			})

			mockFileSystem.EXPECT().Stat(luksDir).Return(nil, nil)
			mockFileSystem.EXPECT().ReadDir(luksDir).Return(files, nil)
			mockCommandBuilder.EXPECT().AddArgs("--key", "all:file:/etc/luks/key1").Return(mockCommandBuilder)

			err := AddLUKSKeys(mockFileSystem, mockCommandBuilder, luksDir)
			Expect(err).ToNot(HaveOccurred())
		})

		It("does nothing when directory does not exist", func() {
			luksDir := "/etc/luks"

			mockFileSystem.EXPECT().Stat(luksDir).Return(nil, os.ErrNotExist)
			// No AddArgs call expected

			err := AddLUKSKeys(mockFileSystem, mockCommandBuilder, luksDir)
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns error when stat returns non-existence error", func() {
			luksDir := "/etc/luks"

			mockFileSystem.EXPECT().Stat(luksDir).Return(nil, errors.New("permission denied"))

			err := AddLUKSKeys(mockFileSystem, mockCommandBuilder, luksDir)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("error accessing the LUKS directory"))
		})

		It("returns error when ReadDir fails", func() {
			luksDir := "/etc/luks"

			mockFileSystem.EXPECT().Stat(luksDir).Return(nil, nil)
			mockFileSystem.EXPECT().ReadDir(luksDir).Return(nil, errors.New("read error"))

			err := AddLUKSKeys(mockFileSystem, mockCommandBuilder, luksDir)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("error reading files in LUKS directory"))
		})

		It("handles empty directory", func() {
			luksDir := "/etc/luks"

			mockFileSystem.EXPECT().Stat(luksDir).Return(nil, nil)
			mockFileSystem.EXPECT().ReadDir(luksDir).Return([]os.DirEntry{}, nil)
			mockCommandBuilder.EXPECT().AddArgs("--key").Return(mockCommandBuilder)

			err := AddLUKSKeys(mockFileSystem, mockCommandBuilder, luksDir)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("GetFilesInPath", func() {
		It("returns file paths from directory", func() {
			rootPath := "/test/path"
			files := ConvertMockDirEntryToOs([]MockDirEntry{
				{FileName: "file1.txt", FileIsDir: false},
				{FileName: "file2.txt", FileIsDir: false},
			})

			mockFileSystem.EXPECT().ReadDir(rootPath).Return(files, nil)

			paths, err := GetFilesInPath(mockFileSystem, rootPath)
			Expect(err).ToNot(HaveOccurred())
			Expect(paths).To(HaveLen(2))
			Expect(paths).To(ContainElement("/test/path/file1.txt"))
			Expect(paths).To(ContainElement("/test/path/file2.txt"))
		})

		It("skips directories", func() {
			rootPath := "/test/path"
			files := ConvertMockDirEntryToOs([]MockDirEntry{
				{FileName: "file1.txt", FileIsDir: false},
				{FileName: "subdir", FileIsDir: true},
			})

			mockFileSystem.EXPECT().ReadDir(rootPath).Return(files, nil)

			paths, err := GetFilesInPath(mockFileSystem, rootPath)
			Expect(err).ToNot(HaveOccurred())
			Expect(paths).To(HaveLen(1))
			Expect(paths).To(ContainElement("/test/path/file1.txt"))
		})

		It("skips files starting with ..", func() {
			rootPath := "/test/path"
			files := ConvertMockDirEntryToOs([]MockDirEntry{
				{FileName: "file1.txt", FileIsDir: false},
				{FileName: "..data", FileIsDir: false},
			})

			mockFileSystem.EXPECT().ReadDir(rootPath).Return(files, nil)

			paths, err := GetFilesInPath(mockFileSystem, rootPath)
			Expect(err).ToNot(HaveOccurred())
			Expect(paths).To(HaveLen(1))
			Expect(paths).To(ContainElement("/test/path/file1.txt"))
		})

		It("returns error when ReadDir fails", func() {
			rootPath := "/test/path"

			mockFileSystem.EXPECT().ReadDir(rootPath).Return(nil, errors.New("read error"))

			paths, err := GetFilesInPath(mockFileSystem, rootPath)
			Expect(err).To(HaveOccurred())
			Expect(paths).To(BeNil())
		})

		It("returns empty slice for empty directory", func() {
			rootPath := "/test/path"

			mockFileSystem.EXPECT().ReadDir(rootPath).Return([]os.DirEntry{}, nil)

			paths, err := GetFilesInPath(mockFileSystem, rootPath)
			Expect(err).ToNot(HaveOccurred())
			Expect(paths).To(BeEmpty())
		})
	})
})
