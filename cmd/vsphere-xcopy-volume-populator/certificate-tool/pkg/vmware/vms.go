package vmware

import (
	"certificate-tool/internal/utils/osutils"
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/vmware/govmomi/vmdk"
	"k8s.io/klog/v2"

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
	Host        string
}

// downloadVMDKIfMissing checks for the VMDK locally, downloading it if absent.
// Returns the local filename of the VMDK.
func ensureVmdk(downloadVmdkURL, localVmdkPath string) (string, error) {
	if downloadVmdkURL == "" {
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
	fmt.Printf("the downloadable vmdk url %+v \n", downloadVmdkURL)
	u, err := url.Parse(downloadVmdkURL)
	if err != nil {
		return "", fmt.Errorf("invalid VMDK URL %q: %w", downloadVmdkURL, err)
	}
	fmt.Printf("the downloadable vmdk url %+v \n", downloadVmdkURL)
	fmt.Printf("the parsed downloadable vmdk url %+v \n", u)
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

	fmt.Printf("the downlaodble dest %v\n", dest)
	pwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return path.Join(pwd, dest), nil
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

func uploadVmdk(
	ctx context.Context,
	client *govmomi.Client,
	ds *object.Datastore,
	dc *object.Datacenter,
	rp *object.ResourcePool,
	host *object.HostSystem,
	vmName string,
	localFilePath string) (string, error) {
	folders, err := dc.Folders(ctx)
	if err != nil {
		return "", fmt.Errorf("cannot get DC folders: %w", err)
	}
	remoteVmdkPath := fmt.Sprintf("[%s] %s/%s", ds.Name(), vmName, filepath.Base(localFilePath))
	log.Printf("Importing vmdk %s\n", remoteVmdkPath)
	err = vmdk.Import(
		ctx,
		client.Client,
		localFilePath,
		ds,
		vmdk.ImportParams{
			Datacenter: dc,
			Pool:       rp,
			Folder:     folders.VmFolder,
			Host:       host,
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

// getExistingVMDKPath queries an existing VM for its primary VMDK path.
// It prioritizes finding the "-flat.vmdk" version if it exists, otherwise, returns the regular VMDK path.
func getExistingVMDKPath(ctx context.Context, vm *object.VirtualMachine, ds *object.Datastore) (string, error) {
	devices, err := vm.Device(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get VM devices: %w", err)
	}

	var vmdkPath string
	for _, device := range devices {
		if disk, ok := device.(*types.VirtualDisk); ok {
			if backing, ok := disk.Backing.(*types.VirtualDiskFlatVer2BackingInfo); ok {
				vmdkPath = backing.FileName
				klog.Infof("Found existing VMDK at %s", vmdkPath)
				break
			}
		}
	}

	if vmdkPath == "" {
		return "", fmt.Errorf("no VMDK found for VM %q", vm.Name())
	}

	parts := strings.SplitN(vmdkPath, "] ", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid VMDK path format: %q", vmdkPath)
	}

	klog.Infof("Found existing regular VMDK for VM %q: %s", vm.Name(), vmdkPath)
	return vmdkPath, nil
}

func createVM(ctx context.Context, cli *govmomi.Client,
	dc *object.Datacenter, rp *object.ResourcePool, host *object.HostSystem, // Add host parameter
	vmName, vmdkPath, dsName string) (*object.VirtualMachine, error) {
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
	task, err := folders.VmFolder.CreateVM(ctx, vmConfig, rp, host)
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

func CreateVM(vmName, vsphereUrl, vsphereUser, vspherePassword, dataCenter,
	dataStore, pool, hostName, downloadVmdkURL, localVmdkPath, isoPath string, waitTimeout time.Duration) (string, error) { // Add hostName parameter
	ctx, cancel, client, finder, dc, ds, rp, err := SetupVSphere(
		5*time.Minute, vsphereUrl, vsphereUser, vspherePassword, dataCenter, dataStore, pool)
	if err != nil {
		log.Fatalf("vSphere setup failed: %v", err)
	}
	defer cancel()

	var host *object.HostSystem
	if hostName != "" {
		host, err = finder.HostSystem(ctx, hostName)
		if err != nil {
			return "", fmt.Errorf("failed to find host %q: %w", hostName, err)
		}
		klog.Infof("Using host: %s", host.Name())
	}

	vm, err := finder.VirtualMachine(context.Background(), vmName)
	if err != nil {
		if _, ok := err.(*find.NotFoundError); !ok {
			return "", err
		}
	}

	if vm != nil {
		log.Printf("VM %q already exists. Attempting to retrieve its VMDK path from vSphere.", vmName)
		existingVmdkPath, err := getExistingVMDKPath(ctx, vm, ds)
		if err != nil {
			return "", fmt.Errorf("failed to get VMDK path for existing VM %q: %w", vmName, err)
		}
		return existingVmdkPath, nil
	}

	vmdkToUpload, err := ensureVmdk(downloadVmdkURL, localVmdkPath)
	if err != nil {
		return "", err
	}
	fmt.Printf("\nvmdk to upload %s\n", vmdkToUpload)
	remoteVmdkPath, err := uploadVmdk(ctx, client, ds, dc, rp, host, vmName, vmdkToUpload)
	if err != nil {
		return "", err
	}
	fmt.Printf("\nremote vmdk path %s\n", remoteVmdkPath)

	// After upload, the `remoteVmdkPath` should correctly point to the descriptor VMDK.
	// We don't need a separate findVMDKPath after upload because uploadVmdk already handles it.
	// The `createVM` function will then use this `remoteVmdkPath`.

	remoteIsoPath, err := uploadFile(ctx, ds, vmName, isoPath)
	if err != nil {
		return "", err
	}
	vm, err = createVM(ctx, client, dc, rp, host, vmName, remoteVmdkPath, ds.Name())
	if err != nil {
		return "", err
	}
	if err := attachCDROM(ctx, vm, remoteIsoPath); err != nil {
		return "", err
	}
	if err := attachNetwork(ctx, client, vm, "VM Network"); err != nil {
		log.Fatalf("add NIC: %v", err)
	}

	if err := waitForVMRegistration(ctx, finder, vmName, waitTimeout); err != nil {
		return "", err
	}

	klog.Infof("VM %s is ready.", vmName)
	return remoteVmdkPath, nil
}

func DestroyVM(vmName, vsphereUrl, vsphereUser, vspherePassword, dataCenter,
	dataStore, pool string, timeout time.Duration) error {
	ctx, cancel, _, finder, _, _, _, err := SetupVSphere(
		timeout, vsphereUrl, vsphereUser, vspherePassword, dataCenter, dataStore, pool)
	if err != nil {
		log.Fatalf("vSphere setup failed: %v", err)
	}
	defer cancel()
	vm, err := finder.VirtualMachine(ctx, vmName)
	if err != nil {
		if _, ok := err.(*find.NotFoundError); ok {
			return nil
		} else {
			return err
		}
	}
	if vm != nil {
		log.Printf("Destroying VM %s", vmName)
		task, err := vm.Destroy(context.Background())
		if err != nil {
			return err
		}

		powerState, err := vm.PowerState(context.Background())
		if err != nil {
			return err
		}
		// --- Power Off the VM if it's On or Suspended ---
		if powerState == types.VirtualMachinePowerStatePoweredOn || powerState == types.VirtualMachinePowerStateSuspended {
			log.Printf("Powering off Virtual Machine '%s' before destruction...", vm.Name())
			task, err := vm.PowerOff(ctx)
			if err != nil {
				log.Fatalf("Failed to initiate power off for VM '%s': %v", vm.Name(), err)
			}

			// Wait for the power-off task to complete
			if err = task.Wait(ctx); err != nil {
				// Log the error but attempt to destroy anyway, as some power-off failures might still allow destruction.
				log.Printf("Warning: Power off task for VM '%s' failed or timed out: %v. Attempting destruction anyway.", vm.Name(), err)
			} else {
				log.Printf("Virtual Machine '%s' powered off successfully.", vm.Name())
			}
		} else {
			log.Printf("Virtual Machine '%s' is already powered off.", vm.Name())
		}

		// Wait for the destroy task to complete
		log.Printf("Waiting for destroy task to complete for VM '%s' (Task ID: %s)...", vm.Name(), task.Reference())
		if err = task.Wait(ctx); err != nil {
			log.Fatalf("Destroy task for VM '%s' failed or timed out: %v", vm.Name(), err)
		}

		log.Printf("Virtual Machine '%s' destroyed successfully!", vm.Name())
		log.Printf("\nSUCCESS: Virtual Machine '%s' has been destroyed.\n", vmName)
	}
	return nil
}
