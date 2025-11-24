package populator

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/vmware"
	"github.com/vmware/govmomi/object"
	"k8s.io/klog/v2"
)

var xcopyInitiatorGroup = "xcopy-esxs"

const (
	taskPollingInterval = 5 * time.Second
	rescanSleepInterval = 5 * time.Second
	rescanRetries       = 5
)

// CloneMethod represents the method used for cloning operations
type CloneMethod string

const (
	// CloneMethodSSH uses SSH to perform cloning operations
	CloneMethodSSH CloneMethod = "ssh"
	// CloneMethodVIB uses VIB to perform cloning operations
	CloneMethodVIB CloneMethod = "vib"
)

type vmkfstoolsClone struct {
	Pid    int    `json:"pid"`
	TaskId string `json:"taskId"`
}

type vmkfstoolsTask struct {
	Pid      int    `json:"pid"`
	ExitCode string `json:"exitCode"`
	Stderr   string `json:"stdErr"`
	LastLine string `json:"lastLine"`
	TaskId   string `json:"taskId"`
}

type EsxCli interface {
	ListVibs() ([]string, error)
	VmkfstoolsClone(sourceVMDKFile, targetLUN string) error
}

type RemoteEsxcliPopulator struct {
	VSphereClient vmware.Client
	StorageApi    StorageApi
	// SSH-related fields (only used when using SSH method)
	SSHPrivateKey []byte
	SSHPublicKey  []byte
	UseSSHMethod  bool
	SSHTimeout    time.Duration
}

func NewWithRemoteEsxcli(storageApi StorageApi, vsphereHostname, vsphereUsername, vspherePassword string) (Populator, error) {
	c, err := vmware.NewClient(vsphereHostname, vsphereUsername, vspherePassword)
	if err != nil {
		return nil, fmt.Errorf("failed to create vmware client: %w", err)
	}
	return &RemoteEsxcliPopulator{
		VSphereClient: c,
		StorageApi:    storageApi,
		UseSSHMethod:  false,            // VIB method
		SSHTimeout:    30 * time.Second, // Default timeout (not used for VIB method)
	}, nil
}

func NewWithRemoteEsxcliSSH(storageApi StorageApi, vsphereHostname, vsphereUsername, vspherePassword string, sshPrivateKey, sshPublicKey []byte, sshTimeoutSeconds int) (Populator, error) {
	c, err := vmware.NewClient(vsphereHostname, vsphereUsername, vspherePassword)
	if err != nil {
		return nil, fmt.Errorf("failed to create vmware client: %w", err)
	}
	if len(sshPrivateKey) == 0 || len(sshPublicKey) == 0 {
		return nil, fmt.Errorf("ssh key material must be non-empty")
	}
	return &RemoteEsxcliPopulator{
		VSphereClient: c,
		StorageApi:    storageApi,
		SSHPrivateKey: sshPrivateKey,
		SSHPublicKey:  sshPublicKey,
		UseSSHMethod:  true,
		SSHTimeout:    time.Duration(sshTimeoutSeconds) * time.Second,
	}, nil
}

func (p *RemoteEsxcliPopulator) Populate(vmId string, sourceVMDKFile string, pv PersistentVolume, progress chan<- uint, quit chan error) (errFinal error) {
	// isn't it better to not call close the channel from the caller?
	defer func() {
		r := recover()
		if r != nil {
			klog.Infof("recovered %v", r)
			// if we paniced we must return with an error. Otherwise, the pod will exit with 0 and will
			// continue to convertion, and will likely fail, if the copy wasn't completed.
			quit <- fmt.Errorf("recovered failure: %v", r)
			return
		}
		quit <- errFinal
	}()
	vmDisk, err := ParseVmdkPath(sourceVMDKFile)
	if err != nil {
		return err
	}

	var cloneMethod CloneMethod
	klog.Infof("Debug: UseSSHMethod field value: %t", p.UseSSHMethod)
	if p.UseSSHMethod {
		cloneMethod = CloneMethodSSH
		klog.Infof("Debug: Set cloneMethod to SSH")
	} else {
		cloneMethod = CloneMethodVIB
		klog.Infof("Debug: Set cloneMethod to VIB")
	}

	klog.Infof(
		"Starting populate via remote esxcli vmkfstools (%s), source vmdk=%s, pv=%v",
		cloneMethod,
		sourceVMDKFile,
		pv)
	host, err := p.VSphereClient.GetEsxByVm(context.Background(), vmId)
	if err != nil {
		return err
	}
	klog.Infof("Got ESXi host: %s", host)

	// Only ensure VIB if using VIB method
	if !p.UseSSHMethod {
		err = ensureVib(p.VSphereClient, host, vmDisk.Datastore, VibVersion)
		if err != nil {
			return fmt.Errorf("failed to ensure VIB is installed: %w", err)
		}
	}

	// for iSCSI add the host to the group using IQN. Is there something else for FC?
	r, err := p.VSphereClient.RunEsxCommand(context.Background(), host, []string{"storage", "core", "adapter", "list"})
	if err != nil {
		return err
	}
	uniqueUIDs := make(map[string]bool)
	hbaUIDs := []string{}

	isSciniRequired := false
	if sciniAware, ok := p.StorageApi.(SciniAware); ok {
		if sciniAware.SciniRequired() {
			isSciniRequired = true
		}
	}

	// powerflex handling - scini is the powerflex kernel module and is not
	// using any iqn/wwn to identity the host. Instead extract the SdcGuid
	// as the possible clonner identifier
	if isSciniRequired {
		klog.Infof("scini is required for the storage api")
		sciModule, err := p.VSphereClient.RunEsxCommand(context.Background(), host, []string{"system", "module", "parameters", "list", "-m", "scini"})
		if err != nil {
			klog.Infof("failed to fetch the scini module parameters %s: ", err)
			return err
		}
		for _, moduleFields := range sciModule {

			if slices.Contains(moduleFields["Name"], "IoctlIniGuidStr") {
				klog.Infof("scini guid %v", moduleFields["Value"])
				for _, s := range moduleFields["Value"] {
					hbaUIDs = append(hbaUIDs, strings.ToUpper(s))
				}
				klog.Infof("Scini hbas found: %+v", hbaUIDs)
			}
		}
	}

	if !isSciniRequired {
		klog.Infof("scini is not required for the storage api")
		for i, a := range r {
			klog.Infof("Adapter [%d]: %+v", i, a)
			for key, field := range a {
				klog.Infof("  %s: %v", key, field)
			}
			driver, hasDriver := a["Driver"]
			if !hasDriver {
				// irrelevant adapter
				continue
			}

			// 'esxcli storage core adapter list' returns LinkState field
			// 'esxcli iscsi adapater list' returns State field
			linkState, hasLink := a["LinkState"]
			uid, hasUID := a["UID"]

			if !hasDriver || !hasLink || !hasUID || len(driver) == 0 || len(linkState) == 0 || len(uid) == 0 {
				continue
			}

			drv := driver[0]
			link := linkState[0]
			id := uid[0]
			id = strings.ToLower(strings.TrimSpace(id))
			// Check if the UID is FC, iSCSI or NVMe-oF
			isTargetUID := strings.HasPrefix(id, "fc.") || strings.HasPrefix(id, "iqn.") || strings.HasPrefix(id, "nqn.")

			if (link == "link-up" || link == "online") && isTargetUID {
				if _, exists := uniqueUIDs[id]; !exists {
					uniqueUIDs[id] = true
					hbaUIDs = append(hbaUIDs, id)
					klog.Infof("Storage Adapter UID: %s (Driver: %s)", id, drv)
				}
			}
		}
		klog.Infof("HBA UIDs found: %+v", hbaUIDs)
	}

	if len(hbaUIDs) == 0 {
		klog.Infof("no valid HBA UIDs found for host %s", host)
		return fmt.Errorf("no valid HBA UIDs found for host %s", host)
	}
	mappingContext, err := p.StorageApi.EnsureClonnerIgroup(xcopyInitiatorGroup, hbaUIDs)
	if err != nil {
		return fmt.Errorf("failed to add the ESX HBA UID %s to the initiator group %w", hbaUIDs, err)
	}

	lun, err := p.StorageApi.ResolvePVToLUN(pv)
	if err != nil {
		return err
	}

	originalInitiatorGroups, err := p.StorageApi.CurrentMappedGroups(lun, mappingContext)
	if err != nil {
		return fmt.Errorf("failed to fetch the current initiator groups of the lun %s: %w", lun.Name, err)
	}
	klog.Infof("Current initiator groups the LUN %s is mapped to %+v", lun.IQN, originalInitiatorGroups)

	if isSciniRequired {
		sdcId, ok := mappingContext["sdcId"]
		if !ok {
			klog.Infof("sdcId is required but not found in mappingContext")
			return fmt.Errorf("sdcId is required but not found in mappingContext")
		} else {
			xcopyInitiatorGroup = sdcId.(string)
			klog.Infof("sdcId found in mappingContext: %s", sdcId)
		}
	}

	fullCleanUpAttempted := false

	defer func() {
		if fullCleanUpAttempted {
			return
		}
		if !slices.Contains(originalInitiatorGroups, xcopyInitiatorGroup) {
			// Only attempt cleanup if lun was successfully resolved
			if lun.Name != "" {
				if mappingContext != nil {
					mappingContext["UnmapAllSdc"] = false
				}
				errUnmap := p.StorageApi.UnMap(xcopyInitiatorGroup, lun, mappingContext)
				if errUnmap != nil {
					klog.Infof("failed to unmap all initiator groups during partial cleanup: %s", errUnmap)
				}
			} else {
				klog.V(2).Infof("Skipping cleanup unmap as LUN was not successfully resolved")
			}
		}
	}()

	lun, err = p.StorageApi.Map(xcopyInitiatorGroup, lun, mappingContext)
	if err != nil {
		return fmt.Errorf("failed to map lun %s to initiator group %s: %w", lun, xcopyInitiatorGroup, err)
	}

	targetLUN := fmt.Sprintf("/vmfs/devices/disks/%s", lun.NAA)
	klog.Infof("resolved lun with IQN %s to lun %s", lun.IQN, targetLUN)

	err = rescan(p.VSphereClient, host, lun.NAA)
	if err != nil {
		return fmt.Errorf("failed to find the device %s after scanning: %w", targetLUN, err)
	}

	defer func() {
		klog.Infof("cleaning up - unmap and rescan to clean dead devices")
		fullCleanUpAttempted = true
		if mappingContext != nil {
			mappingContext["UnmapAllSdc"] = true
			mappingContext[CleanupXcopyInitiatorGroup] = true
		}
		errUnmap := p.StorageApi.UnMap(xcopyInitiatorGroup, lun, mappingContext)
		if errUnmap != nil {
			klog.Errorf("failed in unmap during cleanup, lun %s: %s", lun.Name, errUnmap)
		}
		// map the LUN back to the original OCP worker
		klog.Infof("about to map the volume back to the originalInitiatorGroups, which are: %s", originalInitiatorGroups)
		for _, group := range originalInitiatorGroups {
			_, errMap := p.StorageApi.Map(group, lun, mappingContext)
			if errMap != nil {
				klog.Warningf("failed to map the volume back the original holder - this may cause problems: %v", errMap)
			}
		}
		// unmap devices appear dead in ESX right after they are unmapped, now
		// clean them
		_, errClean := p.VSphereClient.RunEsxCommand(
			context.Background(),
			host,
			[]string{"storage", "core", "adapter", "rescan", "-t", "delete", "-a", "1"})
		if errClean != nil {
			klog.Errorf("failed to delete dead devices: %s", err)
		} else {
			klog.Info("rescan to delete dead devices completed")
		}
	}()

	// Execute the clone using the unified task handling approach
	var executor TaskExecutor
	if p.UseSSHMethod {
		sshSetupCtx, sshCancel := context.WithTimeout(context.Background(), p.SSHTimeout)
		defer sshCancel()

		// Setup secure script
		finalScriptPath, err := ensureSecureScript(sshSetupCtx, p.VSphereClient, host, vmDisk.Datastore)
		if err != nil {
			return fmt.Errorf("failed to ensure secure script: %w", err)
		}
		klog.V(2).Infof("Secure script ready at path: %s", finalScriptPath)

		// Enable SSH access
		err = vmware.EnableSSHAccess(sshSetupCtx, p.VSphereClient, host, p.SSHPrivateKey, p.SSHPublicKey, finalScriptPath)
		if err != nil {
			return fmt.Errorf("failed to enable SSH access: %w", err)
		}

		// Get host IP
		hostIP, err := vmware.GetHostIPAddress(sshSetupCtx, host)
		if err != nil {
			return fmt.Errorf("failed to get host IP address: %w", err)
		}

		// Create SSH client with background context (no timeout for long-running operations)
		sshClient := vmware.NewSSHClient()
		err = sshClient.Connect(context.Background(), hostIP, "root", p.SSHPrivateKey)
		if err != nil {
			return fmt.Errorf("failed to connect via SSH: %w", err)
		}
		defer sshClient.Close()

		klog.V(2).Infof("SSH connection established with restricted commands")
		executor = NewSSHTaskExecutor(sshClient)
	} else {
		executor = NewVIBTaskExecutor(p.VSphereClient)
	}

	// Use unified task execution
	return ExecuteCloneTask(context.Background(), executor, host, vmDisk.Path(), targetLUN, progress)
}

// After mapping a volume the ESX needs a rescan to see the device. ESXs can opt-in to do it automatically
func rescan(client vmware.Client, host *object.HostSystem, targetLUN string) error {
	for i := 1; i <= rescanRetries; i++ {
		_, err := client.RunEsxCommand(context.Background(), host, []string{"storage", "core", "device", "list", "-d", targetLUN})
		if err == nil {
			klog.Infof("found device %s", targetLUN)
			return nil
		} else {
			_, err = client.RunEsxCommand(
				context.Background(), host, []string{"storage", "core", "adapter", "rescan", "-t", "add", "-a", "1"})
			if err != nil {
				klog.Errorf("failed to rescan for adapters, attempt %d/%d due to: %s", i, rescanRetries, err)
			}
			time.Sleep(rescanSleepInterval)
		}
	}

	// last check after the last rescan
	_, err := client.RunEsxCommand(context.Background(), host, []string{"storage", "core", "device", "list", "-d", targetLUN})
	if err == nil {
		klog.Infof("found device %s", targetLUN)
		return nil
	} else {
		return fmt.Errorf("failed to find device %s: %w", targetLUN, err)
	}
}
