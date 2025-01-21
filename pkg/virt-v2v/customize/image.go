package customize

import (
	"embed"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/konveyor/forklift-controller/pkg/virt-v2v/global"
	"github.com/konveyor/forklift-controller/pkg/virt-v2v/utils"
)

//go:embed scripts
var scriptFS embed.FS

type FileSystemTool interface {
	CreateFilesFromFS(dstDir string) error
}

type DomainExecFunc func(args ...string) error

func Run(disks []string, operatingSystem string) error {
	var err error
	fmt.Printf("Customizing disks '%s'\n", disks)
	// Customization for vSphere source.
	t := utils.EmbedTool{Filesystem: &scriptFS}
	// windows
	if strings.Contains(operatingSystem, "win") {
		err = CustomizeWindows(CustomizeDomainExec, disks, global.DIR, &t)
		if err != nil {
			fmt.Println("Error customizing disk image:", err)
			return err
		}
	}

	// Linux
	if !strings.Contains(operatingSystem, "win") {
		err = CustomizeLinux(CustomizeDomainExec, disks, global.DIR, &t)
		if err != nil {
			fmt.Println("Error customizing disk image:", err)
			return err
		}
	}
	return nil
}

// CustomizeDomainExec executes `virt-customize` to customize the image.
//
// Arguments:
//   - extraArgs (...string): The additional arguments which will be appended to the `virt-customize` arguments.
//
// Returns:
//   - error: An error if something goes wrong during the process, or nil if successful.
func CustomizeDomainExec(extraArgs ...string) error {
	args := []string{"--verbose", "--format", "raw"}
	args = append(args, extraArgs...)

	customizeCmd := exec.Command("virt-customize", args...)
	customizeCmd.Stdout = os.Stdout
	customizeCmd.Stderr = os.Stderr

	fmt.Println("exec:", customizeCmd)
	if err := customizeCmd.Run(); err != nil {
		return fmt.Errorf("error executing virt-customize command: %w", err)
	}
	return nil
}
