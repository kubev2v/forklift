package customize

import (
	"embed"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/konveyor/forklift-controller/virt-v2v/pkg/global"
	"github.com/konveyor/forklift-controller/virt-v2v/pkg/utils"
)

//go:embed scripts
var scriptFS embed.FS

type FileSystemTool interface {
	CreateFilesFromFS(dstDir string) error
}

type DomainExecFunc func(args ...string) error

func getVmDiskPaths(domain *utils.OvaVmconfig) []string {
	var resp []string
	for _, disk := range domain.Devices.Disks {
		if disk.Source.File != "" {
			resp = append(resp, disk.Source.File)
		}
	}
	return resp
}

func Run(source string, xmlFilePath string) error {
	domain, err := utils.GetDomainFromXml(xmlFilePath)
	if err != nil {
		fmt.Printf("Error mapping xml to domain: %v\n", err)

		// No customization if we can't parse virt-v2v output.
		return err
	}

	// Get operating system.
	operatingSystem := domain.Metadata.LibOsInfo.V2VOS.ID
	if operatingSystem == "" {
		fmt.Printf("Warning: no operating system found")

		// No customization when no known OS detected.
		return nil
	} else {
		fmt.Printf("Operating System ID: %s\n", operatingSystem)
	}

	// Get domain disks.
	disks := getVmDiskPaths(domain)
	if len(disks) == 0 {
		fmt.Printf("Warning: no V2V domain disks found")

		// No customization when no disks found.
		return nil
	} else {
		fmt.Printf("V2V domain disks: %v\n", disks)
	}

	// TOOD: Test customzie with a OVA
	// Customization for vSphere source.
	if source == global.VSPHERE {
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
