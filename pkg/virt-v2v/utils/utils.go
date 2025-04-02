package utils

import (
	"fmt"
	"os"
	"strings"
)

// AddLUKSKeys checks the LUKS directory for key files and returns the appropriate
// arguments for a 'virt-' command to add these keys.
//
// Returns a slice of strings representing the LUKS key arguments, or an error if
// there's an issue accessing the directory or reading the files.
func AddLUKSKeys(filesystem FileSystem, builder CommandBuilder, luksdir string) error {
	if _, err := filesystem.Stat(luksdir); err == nil {
		files, err := GetFilesInPath(filesystem, luksdir)
		if err != nil {
			return fmt.Errorf("error reading files in LUKS directory: %v", err)
		}
		var luksFiles []string
		for _, file := range files {
			luksFiles = append(luksFiles, fmt.Sprintf("all:file:%s", file))
		}
		builder.AddArgs("--key", luksFiles...)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("error accessing the LUKS directory: %v", err)
	}
	return nil
}

func GetFilesInPath(filesystem FileSystem, rootPath string) (paths []string, err error) {
	files, err := filesystem.ReadDir(rootPath)
	if err != nil {
		fmt.Println("Error reading the files in the directory ", err)
		return
	}
	for _, file := range files {
		if !file.IsDir() && !strings.HasPrefix(file.Name(), "..") {
			paths = append(paths, fmt.Sprintf("%s/%s", rootPath, file.Name()))
		}
	}
	return
}
