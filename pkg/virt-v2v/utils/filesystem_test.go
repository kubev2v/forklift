// Generated-by: Claude
package utils

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("FileSystem", func() {
	var tempDir string
	var fs FileSystemImpl

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "filesystem-test")
		Expect(err).ToNot(HaveOccurred())
		fs = FileSystemImpl{}
	})

	AfterEach(func() {
		os.RemoveAll(tempDir)
	})

	Describe("FileSystemImpl", func() {
		Describe("Symlink", func() {
			It("creates symlink successfully", func() {
				// Create a real file first
				realFile := filepath.Join(tempDir, "realfile.txt")
				err := os.WriteFile(realFile, []byte("content"), 0644)
				Expect(err).ToNot(HaveOccurred())

				linkPath := filepath.Join(tempDir, "link.txt")
				err = fs.Symlink(realFile, linkPath)
				Expect(err).ToNot(HaveOccurred())

				// Verify symlink exists and points to correct file
				target, err := os.Readlink(linkPath)
				Expect(err).ToNot(HaveOccurred())
				Expect(target).To(Equal(realFile))
			})

			It("returns error when link already exists", func() {
				realFile := filepath.Join(tempDir, "realfile.txt")
				err := os.WriteFile(realFile, []byte("content"), 0644)
				Expect(err).ToNot(HaveOccurred())

				linkPath := filepath.Join(tempDir, "link.txt")
				err = fs.Symlink(realFile, linkPath)
				Expect(err).ToNot(HaveOccurred())

				// Try to create same link again
				err = fs.Symlink(realFile, linkPath)
				Expect(err).To(HaveOccurred())
			})

			It("returns error for invalid destination directory", func() {
				linkPath := filepath.Join(tempDir, "nonexistent", "link.txt")
				err := fs.Symlink("/some/file", linkPath)
				Expect(err).To(HaveOccurred())
			})
		})

		Describe("Stat", func() {
			It("returns file info for existing file", func() {
				filePath := filepath.Join(tempDir, "testfile.txt")
				err := os.WriteFile(filePath, []byte("content"), 0644)
				Expect(err).ToNot(HaveOccurred())

				info, err := fs.Stat(filePath)
				Expect(err).ToNot(HaveOccurred())
				Expect(info.Name()).To(Equal("testfile.txt"))
				Expect(info.IsDir()).To(BeFalse())
			})

			It("returns file info for existing directory", func() {
				dirPath := filepath.Join(tempDir, "testdir")
				err := os.Mkdir(dirPath, 0755)
				Expect(err).ToNot(HaveOccurred())

				info, err := fs.Stat(dirPath)
				Expect(err).ToNot(HaveOccurred())
				Expect(info.Name()).To(Equal("testdir"))
				Expect(info.IsDir()).To(BeTrue())
			})

			It("returns error for non-existent path", func() {
				_, err := fs.Stat(filepath.Join(tempDir, "nonexistent"))
				Expect(err).To(HaveOccurred())
				Expect(os.IsNotExist(err)).To(BeTrue())
			})
		})

		Describe("WriteFile", func() {
			It("writes file successfully", func() {
				filePath := filepath.Join(tempDir, "newfile.txt")
				content := []byte("test content")

				err := fs.WriteFile(filePath, content, 0644)
				Expect(err).ToNot(HaveOccurred())

				// Verify content
				readContent, err := os.ReadFile(filePath)
				Expect(err).ToNot(HaveOccurred())
				Expect(readContent).To(Equal(content))
			})

			It("overwrites existing file", func() {
				filePath := filepath.Join(tempDir, "existing.txt")
				err := os.WriteFile(filePath, []byte("old content"), 0644)
				Expect(err).ToNot(HaveOccurred())

				newContent := []byte("new content")
				err = fs.WriteFile(filePath, newContent, 0644)
				Expect(err).ToNot(HaveOccurred())

				readContent, err := os.ReadFile(filePath)
				Expect(err).ToNot(HaveOccurred())
				Expect(readContent).To(Equal(newContent))
			})

			It("returns error for invalid path", func() {
				filePath := filepath.Join(tempDir, "nonexistent", "file.txt")
				err := fs.WriteFile(filePath, []byte("content"), 0644)
				Expect(err).To(HaveOccurred())
			})

			It("writes executable file with correct permissions", func() {
				filePath := filepath.Join(tempDir, "script.sh")
				content := []byte("#!/bin/bash\necho hello")

				err := fs.WriteFile(filePath, content, 0755)
				Expect(err).ToNot(HaveOccurred())

				info, err := os.Stat(filePath)
				Expect(err).ToNot(HaveOccurred())
				// Check executable bit is set
				Expect(info.Mode().Perm() & 0100).To(Equal(os.FileMode(0100)))
			})
		})

		Describe("ReadDir", func() {
			It("reads directory contents", func() {
				// Create some files
				err := os.WriteFile(filepath.Join(tempDir, "file1.txt"), []byte("1"), 0644)
				Expect(err).ToNot(HaveOccurred())
				err = os.WriteFile(filepath.Join(tempDir, "file2.txt"), []byte("2"), 0644)
				Expect(err).ToNot(HaveOccurred())
				err = os.Mkdir(filepath.Join(tempDir, "subdir"), 0755)
				Expect(err).ToNot(HaveOccurred())

				entries, err := fs.ReadDir(tempDir)
				Expect(err).ToNot(HaveOccurred())
				Expect(entries).To(HaveLen(3))

				names := make([]string, len(entries))
				for i, e := range entries {
					names[i] = e.Name()
				}
				Expect(names).To(ContainElements("file1.txt", "file2.txt", "subdir"))
			})

			It("returns empty slice for empty directory", func() {
				emptyDir := filepath.Join(tempDir, "empty")
				err := os.Mkdir(emptyDir, 0755)
				Expect(err).ToNot(HaveOccurred())

				entries, err := fs.ReadDir(emptyDir)
				Expect(err).ToNot(HaveOccurred())
				Expect(entries).To(BeEmpty())
			})

			It("returns error for non-existent directory", func() {
				_, err := fs.ReadDir(filepath.Join(tempDir, "nonexistent"))
				Expect(err).To(HaveOccurred())
			})

			It("returns error when reading a file instead of directory", func() {
				filePath := filepath.Join(tempDir, "file.txt")
				err := os.WriteFile(filePath, []byte("content"), 0644)
				Expect(err).ToNot(HaveOccurred())

				_, err = fs.ReadDir(filePath)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("ConvertMockDirEntryToOs", func() {
		It("converts mock entries to os.DirEntry slice", func() {
			mockEntries := []MockDirEntry{
				{FileName: "file1.txt", FileIsDir: false},
				{FileName: "dir1", FileIsDir: true},
				{FileName: "file2.txt", FileIsDir: false},
			}

			osEntries := ConvertMockDirEntryToOs(mockEntries)
			Expect(osEntries).To(HaveLen(3))

			Expect(osEntries[0].Name()).To(Equal("file1.txt"))
			Expect(osEntries[0].IsDir()).To(BeFalse())
			Expect(osEntries[1].Name()).To(Equal("dir1"))
			Expect(osEntries[1].IsDir()).To(BeTrue())
			Expect(osEntries[2].Name()).To(Equal("file2.txt"))
			Expect(osEntries[2].IsDir()).To(BeFalse())
		})

		It("returns empty slice for empty input", func() {
			osEntries := ConvertMockDirEntryToOs([]MockDirEntry{})
			Expect(osEntries).To(BeEmpty())
		})

		It("returns nil for nil input", func() {
			osEntries := ConvertMockDirEntryToOs(nil)
			Expect(osEntries).To(BeNil())
		})
	})
})
