package cmd

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/guest"
	"github.com/vmware/govmomi/vim25/types"
)

const (
	defaultVmdkURL = "https://cloud-images.ubuntu.com/releases/focal/release/ubuntu-20.04-server-cloudimg-amd64.vmdk"
)

// runCmd is a helper to execute shell commands.
func runCmd(cmdName string, args ...string) error {
	fmt.Printf("Running: %s %v\n", cmdName, args)
	cmd := exec.Command(cmdName, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// createTestVMWithDiskAndISO downloads the VMDK if necessary, uploads it (unless skipped),
// creates the VM, attaches the disk, adds a CDROM with the ISO, and powers the VM on.
func createTestVMWithDiskAndISO(vmdkURL, vmName, isoPath, datastore string, skipUpload bool) error {
	if vmdkURL == "" {
		vmdkURL = defaultVmdkURL
	}
	vmdkFile := filepath.Base(vmdkURL)
	vmdkRemotePath := fmt.Sprintf("%s/%s", vmName, vmdkFile)
	if !skipUpload {
		if _, err := os.Stat(vmdkFile); os.IsNotExist(err) {
			fmt.Println("Downloading VMDK:", vmdkURL)
			if err := runCmd("wget", vmdkURL); err != nil {
				return fmt.Errorf("failed to download VMDK: %w", err)
			}
		} else {
			fmt.Println("VMDK already exists locally, skipping download.")
		}
		if err := runCmd("govc", "import.vmdk", "-ds="+datastore, "-pool=Resources", vmdkFile, vmName); err != nil {
			return fmt.Errorf("failed to upload VMDK: %w", err)
		}
	}

	if err := runCmd("govc", "vm.create",
		"-ds="+datastore,
		"-g=ubuntu64Guest",
		"-m=2048",
		"-c=2",
		"-net=VM Network",
		"-on=false",
		vmName,
	); err != nil {
		return fmt.Errorf("failed to create VM: %w", err)
	}

	if err := runCmd("govc", "vm.disk.attach",
		"-vm="+vmName,
		"-disk="+vmdkRemotePath,
		"-ds="+datastore,
	); err != nil {
		return fmt.Errorf("failed to attach disk: %w", err)
	}

	if err := runCmd("govc", "device.cdrom.add", "-vm="+vmName); err != nil {
		return fmt.Errorf("failed to add cdrom: %w", err)
	}

	if err := runCmd("govc", "device.cdrom.insert",
		"-vm="+vmName,
		"-device=cdrom-3000",
		isoPath,
	); err != nil {
		return fmt.Errorf("failed to insert seed ISO: %w", err)
	}

	if err := runCmd("govc", "device.connect", "-vm="+vmName, "cdrom-3000"); err != nil {
		return fmt.Errorf("failed to connect cdrom: %w", err)
	}

	if err := runCmd("govc", "vm.power", "-on", vmName); err != nil {
		return fmt.Errorf("failed to power on vm: %w", err)
	}

	fmt.Println("VM deployed and running!")
	return nil
}

// changeFileSystem connects to the VM via vCenter and executes a command inside the guest
// to write data (for example, creating a file and writing random bytes).
func changeFileSystem(ctx context.Context, client *govmomi.Client, finder *find.Finder, vmName, guestUser, guestPass string, sizeMB int) error {
	fmt.Println("Changing filesystem inside VM...")
	vm, err := finder.VirtualMachine(ctx, vmName)
	if err != nil {
		fmt.Println("Failed to find VM:", err)
		return fmt.Errorf("failed to find VM: %w", err)
	}
	guestOpsMgr := guest.NewOperationsManager(client.Client, vm.Reference())
	// Guest authentication using provided username and password.
	auth := &types.NamePasswordAuthentication{
		Username: guestUser,
		Password: guestPass,
	}
	procManager, err := guestOpsMgr.ProcessManager(ctx)
	if err != nil {
		return fmt.Errorf("failed to get process manager: %w", err)
	}
	filePath := fmt.Sprintf("/tmp/vm-%s-%s.xcopy", vmName, guestUser)
	command := fmt.Sprintf("-c 'touch %s && dd if=/dev/urandom of=%s bs=1M count=%d 2>&1'", filePath, filePath, sizeMB)
	fmt.Println("Executing command:", command)
	programSpec := types.GuestProgramSpec{
		ProgramPath: "/bin/sh",
		Arguments:   command,
	}
	// Execute the command inside the guest.
	pid, err := procManager.StartProgram(ctx, auth, &programSpec)
	if err != nil {
		fmt.Println("Failed to start program:", err)
		return fmt.Errorf("failed to start guest process: %w", err)
	}
	log.Printf("Started process inside VM with PID: %d", pid)
	return nil
}

// Flags for create-vm command.
var (
	vmdkURL    string
	vmName     string
	isoPath    string
	datastore  string
	skipUpload bool

	guestUser  string
	guestPass  string
	dataSizeMB int

	vcURL  string
	vcUser string
	vcPass string
)

// createVmCmd implements the create-vm subcommand.
var createVmCmd = &cobra.Command{
	Use:   "create-vm",
	Short: "Creates a VM, attaches a disk and writes data to it",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Creating VM with disk and ISO...")
		// Create the VM using govc commands.
		err := createTestVMWithDiskAndISO(vmdkURL, vmName, isoPath, datastore, skipUpload)
		if err != nil {
			panic(err)
		}
		time.Sleep(5 * time.Second)
		// Connect to vCenter to write data inside the guest.
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		u, err := url.Parse(vcURL)
		if err != nil {
			panic(fmt.Sprintf("Error parsing vCenter URL: %v", err))
		}
		u.User = url.UserPassword(vcUser, vcPass)
		client, err := govmomi.NewClient(ctx, u, true)
		if err != nil {
			panic(fmt.Sprintf("Failed to connect to vCenter: %v", err))
		}
		defer client.Logout(ctx)

		finder := find.NewFinder(client.Client, true)
		err = changeFileSystem(ctx, client, finder, vmName, guestUser, guestPass, dataSizeMB)
		if err != nil {
			panic(err)
		}
		fmt.Println("VM created and data written successfully!")
	},
}

func init() {
	RootCmd.AddCommand(createVmCmd)
	createVmCmd.Flags().StringVar(&vmdkURL, "vmdk-url", "", "URL to the VMDK image (default downloads if empty)")
	createVmCmd.Flags().StringVar(&vmName, "vm-name", "", "Name of the VM")
	createVmCmd.Flags().StringVar(&isoPath, "iso-path", "seed.iso", "Path to the ISO file")
	createVmCmd.Flags().StringVar(&datastore, "datastore", "", "Datastore to use")
	createVmCmd.Flags().BoolVar(&skipUpload, "skip-upload", false, "Skip uploading the VMDK if it exists")

	// Flags for guest authentication (for writing data into the VM)
	createVmCmd.Flags().StringVar(&guestUser, "guest-user", "fedora", "Guest OS username")
	createVmCmd.Flags().StringVar(&guestPass, "guest-pass", "password", "Guest OS password")
	createVmCmd.Flags().IntVar(&dataSizeMB, "data-size-mb", 1, "Amount of data (in MB) to write inside the VM")

	// Flags for vCenter connection.
	createVmCmd.Flags().StringVar(&vcURL, "vc-url", "", "vCenter URL")
	createVmCmd.Flags().StringVar(&vcUser, "vc-user", "", "vCenter username")
	createVmCmd.Flags().StringVar(&vcPass, "vc-pass", "", "vCenter password")
}
