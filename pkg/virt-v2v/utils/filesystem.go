package utils

import (
	"os"
	"time"
)

//go:generate mockgen -source=filesystem.go -package=utils -destination=mock_filesystem.go
type FileSystem interface {
	Symlink(oldname, newname string) error
	Stat(name string) (os.FileInfo, error)
	WriteFile(name string, data []byte, perm os.FileMode) error
	ReadDir(name string) ([]os.DirEntry, error)
}

type FileSystemImpl struct{}

func (fs FileSystemImpl) Symlink(oldname, newname string) error {
	return os.Symlink(oldname, newname)
}

func (fs FileSystemImpl) Stat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

func (fs FileSystemImpl) WriteFile(name string, data []byte, perm os.FileMode) error {
	return os.WriteFile(name, data, perm)
}

func (fs FileSystemImpl) ReadDir(name string) ([]os.DirEntry, error) {
	return os.ReadDir(name)
}

type MockDirEntry struct {
	FileName  string
	FileIsDir bool
}

func (m MockDirEntry) Name() string      { return m.FileName }
func (m MockDirEntry) IsDir() bool       { return m.FileIsDir }
func (m MockDirEntry) Type() os.FileMode { return 0 } // Default FileMode
func (m MockDirEntry) Info() (os.FileInfo, error) {
	return MockFileInfo{name: m.FileName, isDir: m.FileIsDir}, nil
}

type MockFileInfo struct {
	name  string
	isDir bool
}

func (m MockFileInfo) Name() string       { return m.name }
func (m MockFileInfo) Size() int64        { return 0 }
func (m MockFileInfo) Mode() os.FileMode  { return os.ModePerm }
func (m MockFileInfo) ModTime() time.Time { return time.Now() }
func (m MockFileInfo) IsDir() bool        { return m.isDir }
func (m MockFileInfo) Sys() interface{}   { return nil }

func ConvertMockDirEntryToOs(scripts []MockDirEntry) []os.DirEntry {
	var responseScripts []os.DirEntry
	for _, script := range scripts {
		responseScripts = append(responseScripts, script)
	}
	return responseScripts
}
