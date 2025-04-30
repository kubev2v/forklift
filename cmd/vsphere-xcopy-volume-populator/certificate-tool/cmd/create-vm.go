package cmd

import (
	"certificate-tool/internal/utils/osutils"
	"context"
	"fmt"
	"github.com/vmware/govmomi/vmdk"
	"k8s.io/klog/v2"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
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

var (
	vmName, isoPath, dataStore     string
	guestID                        string
	dataCenter                     string
	memoryMB, cpus                 int
	network, pool, cdDeviceKey     string
	guestUser, guestPass           string
	dataSizeMB                     int
	waitTimeout                    time.Duration
	downloadVmdkURL, localVmdkPath string
)

// downloadVMDKIfMissing checks for the VMDK locally, downloading it if absent.
// Returns the local filename of the VMDK.
func ensureVmdk(downloadVmdkURL, localVmdkPath string) (string, error) {
	if downloadVmdkURL == "" && localVmdkPath == "" {
		downloadVmdkURL = defaultVMDKURL
	}
	if localVmdkPath != "" {
		if _, err := os.Stat(localVmdkPath); err == nil {
			klog.Infof("Using existing local VMDK: %s", localVmdkPath)
			return localVmdkPath, nil
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("unable to stat local VMDK %q: %w", localVmdkPath, err)
		}
	}
	u, err := url.Parse(downloadVmdkURL)
	if err != nil {
		return "", fmt.Errorf("invalid VMDK URL %q: %w", downloadVmdkURL, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("no local VMDK at %q and %q is not an HTTP URL", localVmdkPath, downloadVmdkURL)
	}
	dest := filepath.Base(u.Path)
	if _, err := os.Stat(dest); os.IsNotExist(err) {
		klog.Infof("Downloading VMDK from %s → %s", downloadVmdkURL, dest)
		if err := osutils.ExecCommand("wget", "-O", dest, downloadVmdkURL); err != nil {
			return "", fmt.Errorf("failed to download VMDK: %w", err)
		}
	} else {
		klog.Infof("Using cached download: %s", dest)
	}

	return dest, nil
}
func fileExist(ctx context.Context, ds *object.Datastore, fullPath string) (bool, error) {
	_, err := ds.Stat(ctx, fullPath)
	if err != nil {
		if strings.Contains(err.Error(), "No such file") || strings.Contains(err.Error(), "No such directory") {
			return false, nil
		}
		return false, fmt.Errorf("stat %q: %w", fullPath, err)
	}
	return true, nil
}

func uploadFile(ctx context.Context, ds *object.Datastore, vmName, localFilePath string) (string, error) {
	remote := filepath.Join(vmName, filepath.Base(localFilePath))
	fullRemotePath := fmt.Sprintf("[%s] %s", ds.Name(), remote)
	exist, err := fileExist(ctx, ds, remote)
	if err != nil {
		return "", fmt.Errorf("error checking remote file %s exist: %w", remote, err)
	}
	if exist == true {
		return fullRemotePath, nil
	}
	if err = ds.UploadFile(ctx, localFilePath, remote, nil); err != nil {
		return "", fmt.Errorf("upload ISO to %s failed: %w", remote, err)
	}
	return fullRemotePath, nil
}

func uploadVmdk(ctx context.Context, client *govmomi.Client, ds *object.Datastore, dc *object.Datacenter, rp *object.ResourcePool, vmName, localFilePath string) (string, error) {
	folders, err := dc.Folders(ctx)
	if err != nil {
		return "", fmt.Errorf("cannot get DC folders: %w", err)
	}
	remoteVmdkPath := fmt.Sprintf("[%s] %s/%s", ds.Name(), vmName, filepath.Base(localVmdkPath))
	err = vmdk.Import(
		ctx,
		client.Client,
		localFilePath,
		ds,
		vmdk.ImportParams{
			Datacenter: dc,
			Pool:       rp,
			Folder:     folders.VmFolder,
			Host:       nil,
			Force:      false,
			Path:       vmName,
			Type:       types.VirtualDiskTypeThin,
			Logger:     nil,
		},
	)
	if err != nil {
		return "", fmt.Errorf("import vmdk: %v", err)
	}
	return remoteVmdkPath, nil
}

func createVM(ctx context.Context, cli *govmomi.Client,
	dc *object.Datacenter, rp *object.ResourcePool, vmName, vmdkPath, dsName string) (*object.VirtualMachine, error) {
	vmxPath := fmt.Sprintf("[%s] %s/%s.vmx", dsName, vmName, vmName)

	vmConfig := types.VirtualMachineConfigSpec{
		Name:     vmName,
		GuestId:  "Fedora64Guest",
		MemoryMB: 2048,
		NumCPUs:  2,
		Files: &types.VirtualMachineFileInfo{
			VmPathName: vmxPath,
		},
	}

	isciController := addDefaultSCSIController(&vmConfig)
	diskBacking := &types.VirtualDiskFlatVer2BackingInfo{}
	diskBacking.FileName = vmdkPath
	diskBacking.DiskMode = string(types.VirtualDiskModePersistent)
	diskBacking.ThinProvisioned = types.NewBool(true)
	unit := int32(0)

	disk := &types.VirtualDisk{
		CapacityInKB:    0,
		CapacityInBytes: 0,
		VirtualDevice: types.VirtualDevice{
			ControllerKey: isciController.Key,
			UnitNumber:    &unit,
			Backing:       diskBacking,
		},
	}

	deviceConfigSpec := &types.VirtualDeviceConfigSpec{
		Operation: types.VirtualDeviceConfigSpecOperationAdd,
		Device:    disk,
	}
	vmConfig.DeviceChange = append(vmConfig.DeviceChange, deviceConfigSpec)
	log.Printf("Creating VM %s...", vmName)
	folders, err := dc.Folders(context.TODO())
	if err != nil {
		panic(err)
	}
	task, err := folders.VmFolder.CreateVM(ctx, vmConfig, rp, nil)
	if err != nil {
		return nil, err
	}

	info, err := task.WaitForResult(ctx, nil)
	if err != nil {
		return nil, err
	}

	log.Printf("created VM %s...", vmName)
	return object.NewVirtualMachine(cli.Client, info.Result.(types.ManagedObjectReference)), nil

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
	cdrom.Connectable = &types.VirtualDeviceConnectInfo{
		StartConnected:    true,
		Connected:         true,
		AllowGuestControl: true,
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

func addDefaultSCSIController(vmConfig *types.VirtualMachineConfigSpec) *types.ParaVirtualSCSIController {
	controller := &types.ParaVirtualSCSIController{
		VirtualSCSIController: types.VirtualSCSIController{
			SharedBus: types.VirtualSCSISharingNoSharing,
			VirtualController: types.VirtualController{
				VirtualDevice: types.VirtualDevice{Key: 3000},
				BusNumber:     0,
			},
		},
	}

	controller.VirtualController = types.VirtualController{}
	controller.VirtualController.Key = 1000
	controller.SharedBus = types.VirtualSCSISharingNoSharing
	controller.VirtualController.BusNumber = 0

	controllerSpec := types.VirtualDeviceConfigSpec{
		Operation: types.VirtualDeviceConfigSpecOperationAdd,
		Device:    controller,
	}

	vmConfig.DeviceChange = append(vmConfig.DeviceChange, &controllerSpec)

	log.Println("Added default LSI Logic SAS controller to VM configuration")
	return controller
}

func attachNetwork(
	ctx context.Context,
	cli *govmomi.Client,
	vm *object.VirtualMachine,
	networkName string,
) error {
	finder := find.NewFinder(cli.Client, false)
	dc, err := finder.DefaultDatacenter(ctx)
	if err != nil {
		return fmt.Errorf("get DC: %w", err)
	}
	finder.SetDatacenter(dc)

	netObj, err := finder.Network(ctx, networkName)
	if err != nil {
		return fmt.Errorf("find network %q: %w", networkName, err)
	}
	devices, err := vm.Device(ctx)
	if err != nil {
		return fmt.Errorf("device list: %w", err)
	}

	backing, err := netObj.EthernetCardBackingInfo(ctx)
	if err != nil {
		return fmt.Errorf("build NIC backing: %w", err)
	}

	nic, err := devices.CreateEthernetCard("vmxnet3", backing)
	if err != nil {
		return fmt.Errorf("create NIC: %w", err)
	}

	nicSpec := &types.VirtualDeviceConfigSpec{
		Operation: types.VirtualDeviceConfigSpecOperationAdd,
		Device:    nic,
	}
	spec := types.VirtualMachineConfigSpec{
		DeviceChange: []types.BaseVirtualDeviceConfigSpec{nicSpec},
	}

	task, err := vm.Reconfigure(ctx, spec)
	if err != nil {
		return fmt.Errorf("reconfigure: %w", err)
	}

	if err = task.Wait(ctx); err != nil {
		return fmt.Errorf("reconfigure task: %w", err)
	}

	return nil
}
func SetupVSphere(
	timeout time.Duration,
	vcURL, user, pass, dcName, dsName, poolName string,
) (
	ctx context.Context,
	cancel context.CancelFunc,
	cli *govmomi.Client,
	finder *find.Finder,
	dc *object.Datacenter,
	ds *object.Datastore,
	rp *object.ResourcePool,
	err error,
) {
	ctx, cancel = context.WithTimeout(context.Background(), timeout)
	u, err := url.Parse(vcURL)
	if err != nil {
		err = fmt.Errorf("invalid vCenter URL: %w", err)
		return
	}
	u.User = url.UserPassword(user, pass)
	cli, err = govmomi.NewClient(ctx, u, true /* allowInsecure */)
	if err != nil {
		err = fmt.Errorf("vCenter connect error: %w", err)
		return
	}
	finder = find.NewFinder(cli.Client, false)
	dc, err = finder.Datacenter(ctx, dcName)
	if err != nil {
		err = fmt.Errorf("find datacenter %q: %w", dcName, err)
		return
	}
	finder.SetDatacenter(dc)
	ds, err = finder.Datastore(ctx, dsName)
	if err != nil {
		err = fmt.Errorf("find datastore %q: %w", dsName, err)
		return
	}
	rp, err = finder.ResourcePool(ctx, poolName)
	if err != nil {
		err = fmt.Errorf("find resource pool %q: %w", dsName, err)
		return
	}

	return
}

var createVmCmd = &cobra.Command{
	Use:   "create-vm",
	Short: "Create a VM, attach ISO, and inject data into guest",
	RunE: func(cmd *cobra.Command, args []string) error {
		vmCfg := VMConfig{GuestID: guestID, MemoryMB: memoryMB, CPUs: cpus, Network: network, Pool: pool, CDDeviceKey: cdDeviceKey}
		ctx, cancel, client, finder, dc, ds, rp, err := SetupVSphere(
			5*time.Minute, vsphereUrl, vsphereUser, vspherePassword, dataCenter, dataStore, vmCfg.Pool)
		if err != nil {
			log.Fatalf("vSphere setup failed: %v", err)
		}
		defer cancel()
		_, err = ensureVmdk(downloadVmdkURL, localVmdkPath)
		if err != nil {
			return err
		}
		remoteVmdkPath, err := uploadVmdk(ctx, client, ds, dc, rp, vmName, localVmdkPath)
		if err != nil {
			return err
		}
		remoteIsoPath, err := uploadFile(ctx, ds, vmName, isoPath)
		if err != nil {
			return err
		}
		vm, err := createVM(ctx, client, dc, rp, vmName, remoteVmdkPath, ds.Name())
		if err != nil {
			return err
		}
		if err := attachCDROM(ctx, vm, remoteIsoPath); err != nil {
			return err
		}
		if err := attachNetwork(ctx, client, vm, "VM Network"); err != nil {
			log.Fatalf("add NIC: %v", err)
		}
		//>>>>>>>>>>>>>>>>>>>>>>VM must be shut down for some reason<<<<<<<<<<<<<<<<<<<<<<<
		//if err := powerOn(ctx, vm); err != nil {
		//	return err
		//}

		if err := waitForVMRegistration(ctx, finder, vmName, waitTimeout); err != nil {
			return err
		}
		//>>>>>>>>>>>>>>>>>>>>>>This step doesnt work until manual log in to the new vm (idk why)<<<<<<<<<<<<<<<<<<<<<<<
		//if err := writeRandomDataToGuest(ctx, client, finder, vmName, guestUser, guestPass, dataSizeMB); err != nil {
		//	return err
		//}

		klog.Infof("VM %s is ready.", vmName)
		return nil
	},
}

func init() {
	RootCmd.AddCommand(createVmCmd)

	// Image / source flags
	createVmCmd.Flags().StringVar(&downloadVmdkURL, "download-vmdk-url", "", "URL to download VMDK from (defaults to Ubuntu 20.04)")
	createVmCmd.Flags().StringVar(&localVmdkPath, "local-vmdk-path", "assets/cloudinit/ubuntu-20.04-server-cloudimg-amd64.vmdk", "Path to an already-downloaded VMDK file")
	createVmCmd.Flags().StringVar(&vmName, "vm-name", "", "Name for the new VM")
	createVmCmd.Flags().StringVar(&isoPath, "iso-path", "assets/cloudinit/seed.iso", "ISO path to attach as CD‑ROM")
	createVmCmd.Flags().StringVar(&dataStore, "data-store", "", "Target dataStore name")
	createVmCmd.Flags().StringVar(&dataCenter, "data-center", "", "Target dataStore name")
	createVmCmd.Flags().StringVar(&guestID, "guest-id", "ubuntu64Guest", "VM guest ID")
	createVmCmd.Flags().IntVar(&memoryMB, "memory-mb", 2048, "Memory size (MB)")
	createVmCmd.Flags().IntVar(&cpus, "cpus", 2, "vCPU count")
	createVmCmd.Flags().StringVar(&network, "network", "VM Network", "Network name")
	createVmCmd.Flags().StringVar(&pool, "pool", "Resources", "Resource pool path")
	createVmCmd.Flags().StringVar(&cdDeviceKey, "cd-device-key", "cdrom-3000", "Virtual CD‑ROM device key")
	createVmCmd.Flags().StringVar(&guestUser, "guest-user", "fedora", "Guest OS user")
	createVmCmd.Flags().StringVar(&guestPass, "guest-pass", "password", "Guest OS password")
	createVmCmd.Flags().IntVar(&dataSizeMB, "data-size-mb", 1, "Random data size (MB) to write inside guest")
	createVmCmd.Flags().DurationVar(&waitTimeout, "wait-timeout", 10*time.Minute, "Timeout for vCenter operations")

}
