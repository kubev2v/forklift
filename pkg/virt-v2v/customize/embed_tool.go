package customize

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
)

//go:generate mockgen -source=embed_tool.go -package=customize -destination=mock_embed_tool.go
type EmbedTool interface {
	CreateFilesFromFS(dstDir string) error
}

// EmbedToolImpl for manipulating the embedded Filesystem
type EmbedToolImpl struct {
	Filesystem *embed.FS
}

// CreateFilesFromFS gets all files from the embedded Filesystem and recreates them on the disk.
// It creates all directories and keeps the hierarchy of the embedded files.
//
// Arguments:
//   - dstDir (string): The path where the files should be created
//
// Returns:
//   - error: An error if the file cannot be read, or nil if successful.
func (t *EmbedToolImpl) CreateFilesFromFS(dstDir string) error {
	files, err := t.getAllFilenames()
	if err != nil {
		return err
	}
	fmt.Println("Writing files from embedded to the disk")
	for _, file := range files {
		dstFilePath := filepath.Join(dstDir, file)
		fmt.Printf("Writing file from: '%s' to '%s'\n", file, dstFilePath)
		err = t.writeFileFromFS(file, dstFilePath)
		if err != nil {
			return err
		}
	}
	return nil
}

// writeFileFromFS writes a file from the embedded Filesystem to the disk.
//
// Arguments:
//   - src (string): The filepath from the embedded Filesystem which should be writen to the disk.
//   - dst (string): The destination path on the host Filesystem to which the path should be writen.
//
// Returns:
//   - error: An error if the file cannot be read, or nil if successful.
func (t *EmbedToolImpl) writeFileFromFS(src, dst string) error {
	// Create destination directory to the destination if missing
	dstDir := filepath.Dir(dst)
	if _, err := os.Stat(dstDir); os.IsNotExist(err) {
		err := os.MkdirAll(dstDir, 0755)
		if err != nil {
			fmt.Println("Failed creating the directory:", dstDir)
			return err
		}
	}
	// Read the embedded file
	srcData, err := t.Filesystem.ReadFile(src)
	if err != nil {
		fmt.Println("Error reading embedded file")
		return err
	}
	// Write the script to the specified file path
	err = os.WriteFile(dst, srcData, 0755)
	if err != nil {
		return err
	}
	return nil
}

// getAllFilenames gets all files located inside the embedded filesystem.
// Example of one path `scripts/windows/9999-run-mtv-ps-scripts.bat`.
//
// Returns:
//   - []files: The file paths which are located inside the embedded Filesystem.
//   - error: An error if the file cannot be read, or nil if successful.
func (t *EmbedToolImpl) getAllFilenames() (files []string, err error) {
	var nameExcludeChars = regexp.MustCompile(".*test.*")
	if err := fs.WalkDir(t.Filesystem, ".", func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return nil
		}
		if nameExcludeChars.MatchString(path) {
			return nil
		}
		files = append(files, path)
		return nil
	}); err != nil {
		return nil, err
	}
	return files, nil
}
