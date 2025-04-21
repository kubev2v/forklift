package cmd

import (
	"certificate-tool/internal/utils/osutils"
	"context"
	"fmt"
	"k8s.io/klog/v2"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/guest"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
)

const (
	// defaultVMDKURL is the fallback URL for Ubuntu 20.04 server VMDK
	defaultVMDKURL = "https://cloud-images.ubuntu.com/releases/focal/release/ubuntu-20.04-server-cloudimg-amd64.vmdk"
)

// VMConfig holds parameters for provisioning a VM.
type VMConfig struct {
	GuestID     string
	MemoryMB    int
	CPUs        int
	Network     string
	Pool        string
	CDDeviceKey string
}

// downloadVMDKIfMissing checks for the VMDK locally, downloading it if absent.
// Returns the local filename of the VMDK.
func downloadVMDKIfMissing(vmdkURL string) (string, error) {
	if vmdkURL == "" {
		vmdkURL = defaultVMDKURL
	}
	file := filepath.Base(vmdkURL)
	if _, err := os.Stat(file); os.IsNotExist(err) {
		klog.Infof("Downloading VMDK from %s", vmdkURL)
		if err := osutils.ExecCommand("wget", vmdkURL); err != nil {
			return "", fmt.Errorf("failed to download VMDK: %w", err)
		}
	} else {
		klog.Infof("VMDK already present locally: %s", file)
	}
	return file, nil
}

// uploadVMDK uploads a local VMDK file to the datastore under the VM folder.
func uploadVMDK(ctx context.Context, client *govmomi.Client, dsName, vmName, localFile string) error {
	finder := find.NewFinder(client.Client, false)
	dc, err := finder.DefaultDatacenter(ctx)
	if err != nil {
		return fmt.Errorf("find datacenter: %w", err)
	}
	finder.SetDatacenter(dc)

	ds, err := finder.Datastore(ctx, dsName)
	if err != nil {
		return fmt.Errorf("find datastore %s: %w", dsName, err)
	}

	remotePath := ds.Path(filepath.Join(vmName, filepath.Base(localFile)))

	if err := ds.UploadFile(ctx, localFile, remotePath, nil); err != nil {
		return fmt.Errorf("upload VMDK: %w", err)
	}

	klog.Infof("Uploaded VMDK to %s", remotePath)
	return nil
}
func createVM(ctx context.Context, client *govmomi.Client, cfg VMConfig, datastore, vmName string) (*object.VirtualMachine, error) {
	finder := find.NewFinder(client.Client, false)

	dc, err := finder.DefaultDatacenter(ctx)
	if err != nil {
		return nil, fmt.Errorf("default datacenter: %w", err)
	}
	finder.SetDatacenter(dc)

	pool, err := finder.ResourcePool(ctx, cfg.Pool)
	if err != nil {
		return nil, fmt.Errorf("resource pool %s: %w", cfg.Pool, err)
	}

	spec := types.VirtualMachineConfigSpec{
		Name:     vmName,
		GuestId:  cfg.GuestID,
		NumCPUs:  int32(cfg.CPUs),
		MemoryMB: int64(cfg.MemoryMB),
	}

	folders, err := dc.Folders(ctx)
	if err != nil {
		return nil, fmt.Errorf("get dc folders: %w", err)
	}

	task, err := folders.VmFolder.CreateVM(ctx, spec, pool, nil)
	if err != nil {
		return nil, fmt.Errorf("create VM task: %w", err)
	}
	info, err := task.WaitForResult(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("create VM: %w", err)
	}

	vm := object.NewVirtualMachine(client.Client, info.Result.(types.ManagedObjectReference))
	klog.Infof("Created VM %s", vmName)
	return vm, nil
}

func attachCDROM(ctx context.Context, vm *object.VirtualMachine, isoPath string) error {
	devices, err := vm.Device(ctx)
	if err != nil {
		return fmt.Errorf("list devices: %w", err)
	}

	var controller types.BaseVirtualController
	if ide, err := devices.FindIDEController(""); err == nil {
		controller = ide
	} else if sata, err := devices.FindSATAController(""); err == nil {
		controller = sata
	} else {
		return fmt.Errorf("no IDE or SATA controller found: %w", err)
	}
	cdrom, err := devices.CreateCdrom(controller)
	if err != nil {
		return fmt.Errorf("create cdrom device: %w", err)
	}

	cdrom.Backing = &types.VirtualCdromIsoBackingInfo{
		VirtualDeviceFileBackingInfo: types.VirtualDeviceFileBackingInfo{
			FileName: isoPath,
		},
	}

	cdSpec := &types.VirtualDeviceConfigSpec{
		Operation: types.VirtualDeviceConfigSpecOperationAdd,
		Device:    cdrom,
	}

	spec := types.VirtualMachineConfigSpec{DeviceChange: []types.BaseVirtualDeviceConfigSpec{cdSpec}}

	task, err := vm.Reconfigure(ctx, spec)
	if err != nil {
		return fmt.Errorf("reconfigure VM: %w", err)
	}
	if err := task.Wait(ctx); err != nil {
		return fmt.Errorf("wait reconfigure: %w", err)
	}

	klog.Infof("Attached ISO %s", isoPath)
	return nil
}

// powerOn powers on the VM.
func powerOn(ctx context.Context, vm *object.VirtualMachine) error {
	task, err := vm.PowerOn(ctx)
	if err != nil {
		return fmt.Errorf("start power-on task: %w", err)
	}
	if err := task.Wait(ctx); err != nil {
		return fmt.Errorf("power-on failed: %w", err)
	}
	klog.Infof("Powered on VM %s", vm.InventoryPath)
	return nil
}

// waitForVMRegistration polls until the VM is found in inventory.
func waitForVMRegistration(ctx context.Context, finder *find.Finder, vmName string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := finder.VirtualMachine(ctx, vmName); err == nil {
			return nil
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("timeout waiting for VM %s registration", vmName)
}

// writeRandomDataToGuest starts a dd process inside the guest.
func writeRandomDataToGuest(ctx context.Context, client *govmomi.Client, finder *find.Finder, vmName, user, pass string, mb int) error {
	vm, err := finder.VirtualMachine(ctx, vmName)
	if err != nil {
		return fmt.Errorf("cannot find VM %s: %w", vmName, err)
	}

	opMgr := guest.NewOperationsManager(client.Client, vm.Reference())
	auth := &types.NamePasswordAuthentication{Username: user, Password: pass}
	procMgr, err := opMgr.ProcessManager(ctx)
	if err != nil {
		return fmt.Errorf("guest process manager error: %w", err)
	}

	path := fmt.Sprintf("/tmp/%s-data.bin", vmName)
	spec := types.GuestProgramSpec{ProgramPath: "/bin/dd", Arguments: fmt.Sprintf("if=/dev/urandom of=%s bs=1M count=%d", path, mb)}
	if _, err := procMgr.StartProgram(ctx, auth, &spec); err != nil {
		return fmt.Errorf("start guest write process: %w", err)
	}
	klog.Infof("Random data write started inside VM %s: %d MB to %s", vmName, mb, path)
	return nil
}

var (
	vmdkURL, vmName, isoPath, datastore string
	skipUpload                          bool
	guestID                             string
	memoryMB, cpus                      int
	network, pool, cdDeviceKey          string
	vcURL, vcUser, vcPass               string
	guestUser, guestPass                string
	dataSizeMB                          int
	waitTimeout                         time.Duration
)

var createVmCmd = &cobra.Command{
	Use:   "create-vm",
	Short: "Create a VM, attach ISO, and inject data into guest",
	RunE: func(cmd *cobra.Command, args []string) error {
		localFile, err := downloadVMDKIfMissing(vmdkURL)
		if err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(context.Background(), waitTimeout)
		defer cancel()

		u, err := url.Parse(vcURL)
		if err != nil {
			return fmt.Errorf("invalid vCenter URL: %w", err)
		}
		u.User = url.UserPassword(vcUser, vcPass)
		client, err := govmomi.NewClient(ctx, u, true)
		if err != nil {
			return fmt.Errorf("vCenter connect error: %w", err)
		}

		finder := find.NewFinder(client.Client, false)
		dc, err := finder.DefaultDatacenter(ctx)
		if err != nil {
			return fmt.Errorf("find datacenter: %w", err)
		}
		finder.SetDatacenter(dc)

		if !skipUpload {
			if err := uploadVMDK(ctx, client, datastore, vmName, localFile); err != nil {
				return err
			}
		}

		vmCfg := VMConfig{GuestID: guestID, MemoryMB: memoryMB, CPUs: cpus, Network: network, Pool: pool, CDDeviceKey: cdDeviceKey}
		vm, err := createVM(ctx, client, vmCfg, datastore, vmName)
		if err != nil {
			return err
		}

		if err := attachCDROM(ctx, vm, isoPath); err != nil {
			return err
		}

		if err := powerOn(ctx, vm); err != nil {
			return err
		}

		if err := waitForVMRegistration(ctx, finder, vmName, waitTimeout); err != nil {
			return err
		}

		if err := writeRandomDataToGuest(ctx, client, finder, vmName, guestUser, guestPass, dataSizeMB); err != nil {
			return err
		}

		klog.Infof("VM %s is ready.", vmName)
		return nil
	},
}

func init() {
	RootCmd.AddCommand(createVmCmd)

	// Image / source flags
	createVmCmd.Flags().StringVar(&vmdkURL, "vmdk-url", "", "VMDK URL (defaults to Ubuntu 20.04)")
	createVmCmd.Flags().StringVar(&vmName, "vm-name", "", "Name for the new VM")
	createVmCmd.Flags().StringVar(&isoPath, "iso-path", "assets/cloudinit/seed.iso", "ISO path to attach as CD‑ROM")
	createVmCmd.Flags().StringVar(&datastore, "datastore", "", "Target datastore name")
	createVmCmd.Flags().BoolVar(&skipUpload, "skip-upload", false, "Skip uploading the VMDK if it already exists in datastore")

	// VM size & placement
	createVmCmd.Flags().StringVar(&guestID, "guest-id", "ubuntu64Guest", "VM guest ID")
	createVmCmd.Flags().IntVar(&memoryMB, "memory-mb", 2048, "Memory size (MB)")
	createVmCmd.Flags().IntVar(&cpus, "cpus", 2, "vCPU count")
	createVmCmd.Flags().StringVar(&network, "network", "VM Network", "Network name")
	createVmCmd.Flags().StringVar(&pool, "pool", "Resources", "Resource pool path")
	createVmCmd.Flags().StringVar(&cdDeviceKey, "cd-device-key", "cdrom-3000", "Virtual CD‑ROM device key")

	// Credentials / connection
	createVmCmd.Flags().StringVar(&vcURL, "vsphereUrl", "", "vCenter URL (e.g. https://vc/sdk)")
	createVmCmd.Flags().StringVar(&vcUser, "vsphereUser", "", "vCenter username")
	createVmCmd.Flags().StringVar(&vcPass, "vspherePassword", "", "vCenter password")
	createVmCmd.Flags().StringVar(&guestUser, "guest-user", "fedora", "Guest OS user")
	createVmCmd.Flags().StringVar(&guestPass, "guest-pass", "password", "Guest OS password")

	// Misc
	createVmCmd.Flags().IntVar(&dataSizeMB, "data-size-mb", 1, "Random data size (MB) to write inside guest")
	createVmCmd.Flags().DurationVar(&waitTimeout, "wait-timeout", 2*time.Minute, "Timeout for vCenter operations")

	// Required flags for safety
	createVmCmd.MarkFlagRequired("vm-name")
	createVmCmd.MarkFlagRequired("datastore")
	createVmCmd.MarkFlagRequired("vsphereUrl")
	createVmCmd.MarkFlagRequired("vsphereUser")
	createVmCmd.MarkFlagRequired("vspherePassword")
}
