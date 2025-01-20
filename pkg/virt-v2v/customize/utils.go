package customize

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/konveyor/forklift-controller/pkg/virt-v2v/utils"
)

// getScriptsWithSuffix retrieves all scripts with suffix from the specified directory
func getScriptsWithSuffix(directory string, suffix string) ([]string, error) {
	files, err := os.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("failed to read scripts directory: %w", err)
	}

	var scripts []string
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), suffix) && !strings.HasPrefix(file.Name(), "test-") {
			scriptPath := filepath.Join(directory, file.Name())
			scripts = append(scripts, scriptPath)
		}
	}

	return scripts, nil
}

// addDisksToCustomize appends disk arguments to extraArgs
func addDisksToCustomize(extraArgs *[]string, disks []string) {
	*extraArgs = append(*extraArgs, utils.GetScriptArgs("add", disks...)...)
}

func formatUpload(src string, dst string) string {
	return fmt.Sprintf("%s:%s", src, dst)
}

// getScriptsWithRegex retrieves all scripts with suffix from the specified directory
func getScriptsWithRegex(directory string, regex string) ([]string, error) {
	files, err := os.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("failed to read scripts directory: %w", err)
	}

	r := regexp.MustCompile(regex)
	var scripts []string
	for _, file := range files {
		if !file.IsDir() && r.MatchString(file.Name()) {
			scriptPath := filepath.Join(directory, file.Name())
			scripts = append(scripts, scriptPath)
		}
	}
	return scripts, nil
}
