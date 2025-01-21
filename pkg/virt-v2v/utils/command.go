package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/konveyor/forklift-controller/pkg/virt-v2v/global"
)

func CheckEnvVariablesSet(envVars ...string) bool {
	for _, v := range envVars {
		if os.Getenv(v) == "" {
			return false
		}
	}
	return true
}

// GetScriptArgs generates a list of arguments.
//
// Arguments:
//   - argName (string): Argument name which should be used for all the values
//   - values (...string): The list of values which should be joined with argument names.
//
// Returns:
//   - []string: List of arguments
//
// Example:
//   - getScriptArgs("firstboot", boot1, boot2) => ["--firstboot", boot1, "--firstboot", boot2]
func GetScriptArgs(argName string, values ...string) []string {
	var args []string
	for _, val := range values {
		args = append(args, fmt.Sprintf("--%s", argName), val)
	}
	return args
}

// AddLUKSKeys checks the LUKS directory for key files and returns the appropriate
// arguments for a 'virt-' command to add these keys.
//
// Returns a slice of strings representing the LUKS key arguments, or an error if
// there's an issue accessing the directory or reading the files.
func AddLUKSKeys() ([]string, error) {
	var luksArgs []string

	if _, err := os.Stat(global.LUKSDIR); err == nil {
		files, err := GetFilesInPath(global.LUKSDIR)
		if err != nil {
			return nil, fmt.Errorf("Error reading files in LUKS directory: %v", err)
		}

		var luksFiles []string
		for _, file := range files {
			luksFiles = append(luksFiles, fmt.Sprintf("all:file:%s", file))
		}

		luksArgs = append(luksArgs, GetScriptArgs("key", luksFiles...)...)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("Error accessing the LUKS directory: %v", err)
	}

	return luksArgs, nil
}

func GetFilesInPath(rootPath string) (paths []string, err error) {
	files, err := os.ReadDir(rootPath)
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

func genName(diskNum int) string {
	if diskNum <= 0 {
		return ""
	}

	index := (diskNum - 1) % global.LETTERS_LENGTH
	cycles := (diskNum - 1) / global.LETTERS_LENGTH

	return genName(cycles) + string(global.LETTERS[index])
}

func getDiskNumber(kind global.MountPath, disk string) (int, error) {
	switch kind {
	case global.FS:
		return strconv.Atoi(disk[15:])
	case global.BLOCK:
		return strconv.Atoi(disk[10:])
	default:
		return 0, fmt.Errorf("wrong kind when specifying")
	}
}

func GetDiskName() string {
	if name := os.Getenv("V2V_NewName"); name != "" {
		return name
	}
	return os.Getenv("V2V_vmName")
}

func getDiskLink(kind global.MountPath, disk string) (string, error) {
	diskNum, err := getDiskNumber(kind, disk)
	if err != nil {
		fmt.Println("Error getting disks names ", err)
		return "", err
	}
	return filepath.Join(
		global.DIR,
		fmt.Sprintf("%s-sd%s", GetDiskName(), genName(diskNum+1)),
	), nil
}

func GetLinkedDisks() ([]string, error) {
	disks, err := filepath.Glob(
		filepath.Join(
			global.DIR,
			fmt.Sprintf("%s-sd*", GetDiskName()),
		),
	)
	if err != nil {
		return nil, err
	}
	if len(disks) != 0 {
		return disks, nil
	}
	return nil, fmt.Errorf("no disks founds")
}

func LinkDisks(path global.MountPath) (err error) {
	disks, err := filepath.Glob(string(path))
	if err != nil {
		fmt.Println("Error getting disks ", err)
		return
	}

	for _, disk := range disks {
		diskLink, err := getDiskLink(path, disk)
		if err != nil {
			fmt.Println("Error getting disks names ", err)
			return err
		}
		if path == global.FS {
			disk = fmt.Sprintf("%s/disk.img", disk)
		}
		if err = os.Symlink(disk, diskLink); err != nil {
			fmt.Println("Error creating disk link ", err)
			return err
		}
	}
	return
}
