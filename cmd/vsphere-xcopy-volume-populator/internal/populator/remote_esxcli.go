package populator

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"slices"
	"strings"
	"time"

	version "github.com/hashicorp/go-version"
	vmkfstoolswrapper "github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/vmkfstools-wrapper"
	"github.com/kubev2v/forklift/pkg/lib/util"
	"github.com/kubev2v/forklift/pkg/lib/vsphere_offload"
	"github.com/kubev2v/forklift/pkg/lib/vsphere_offload/vmware"
	"github.com/vmware/govmomi/object"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
)

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

type vmkfstoolsTask struct {
	Pid       int    `json:"pid"`
	ExitCode  string `json:"exitCode"`
	Stderr    string `json:"stdErr"`
	LastLine  string `json:"lastLine"`
	XcopyUsed *bool  `json:"xcopyUsed"` // nil means unknown/not determined
	TaskId    string `json:"taskId"`
}

type EsxCli interface {
	ListVibs() ([]string, error)
	VmkfstoolsClone(sourceVMDKFile, targetLUN string) error
}

type RemoteEsxcliPopulator struct {
	VSphereClient vmware.Client
	StorageApi    VMDKCapable
	// SSH-related fields (only used when using SSH method)
	SSHPrivateKey []byte
	SSHPublicKey  []byte
	UseSSHMethod  bool
	SSHTimeout    time.Duration
}

func NewWithRemoteEsxcli(storageApi VMDKCapable, vmwareClient vmware.Client) (Populator, error) {
	return &RemoteEsxcliPopulator{
		VSphereClient: vmwareClient,
		StorageApi:    storageApi,
		UseSSHMethod:  false,            // VIB method
		SSHTimeout:    30 * time.Second, // Default timeout (not used for VIB method)
	}, nil
}

func NewWithRemoteEsxcliSSH(storageApi VMDKCapable, vmwareClient vmware.Client, sshPrivateKey, sshPublicKey []byte, sshTimeoutSeconds int) (Populator, error) {
	if len(sshPrivateKey) == 0 || len(sshPublicKey) == 0 {
		return nil, fmt.Errorf("ssh key material must be non-empty")
	}
	return &RemoteEsxcliPopulator{
		VSphereClient: vmwareClient,
		StorageApi:    storageApi,
		SSHPrivateKey: sshPrivateKey,
		SSHPublicKey:  sshPublicKey,
		UseSSHMethod:  true,
		SSHTimeout:    time.Duration(sshTimeoutSeconds) * time.Second,
	}, nil
}

func (p *RemoteEsxcliPopulator) Populate(vmId string, sourceVMDKFile string, pv PersistentVolume, hostLocker Hostlocker, progress chan<- uint64, xcopyUsed chan<- int, quit chan error) (errFinal error) {
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
	if p.UseSSHMethod {
		cloneMethod = CloneMethodSSH
	} else {
		cloneMethod = CloneMethodVIB
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

	hostID := strings.ReplaceAll(strings.ToLower(host.String()), ":", "-")
	xcopyInitiatorGroup := fmt.Sprintf("xcopy-%s", hostID)
	klog.Infof("Using per-host initiator group: %s", xcopyInitiatorGroup)

	// for iSCSI add the host to the group using IQN. Is there something else for FC?
	r, err := p.VSphereClient.RunEsxCommand(context.Background(), host, []string{"storage", "core", "adapter", "list"})
	if err != nil {
		return err
	}
	uniqueUIDs := make(map[string]bool)
	hbaUIDs := []string{}
	hbaUIDsNamesMap := make(map[string]string)
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
		for _, a := range r {
			hbaName, hasHbaName := a["HBAName"]
			if !hasHbaName {
				continue
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
					hbaUIDsNamesMap[id] = hbaName[0]
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

	leaseHostID := strings.ReplaceAll(strings.ToLower(host.String()), ":", "-")
	err = hostLocker.WithLock(context.Background(), leaseHostID,
		func(ctx context.Context) error {
			return rescan(ctx, p.VSphereClient, host, lun.NAA)
		},
	)

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

		klog.Errorf("cleaning up lun %s:", lun.NAA)
		// set device state to off and prevents any i/o to it
		_, err = p.VSphereClient.RunEsxCommand(context.Background(), host, []string{"storage", "core", "device", "set", "--state", "off", "-d", lun.NAA})
		if err != nil {
			klog.Errorf("failed to set state off for device %s: %s", lun.Name, err)
		} else {
			// Wait for the device state to become "off" using exponential backoff
			err = waitForDeviceStateOff(p.VSphereClient, host, lun.NAA)
			if err != nil {
				klog.Errorf("timeout waiting for device %s to reach off state: %s", lun.Name, err)
			}
		}
		_, err = p.VSphereClient.RunEsxCommand(context.Background(), host, []string{"storage", "core", "device", "detached", "remove", "-d", lun.NAA})
		if err != nil {
			klog.Errorf("failed to remove device from detached list %s: %s", lun.Name, err)
		}
		// finaly after the kernel have it detached and not having any i/o we can unmap
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
		klog.Infof("about to delete dead devices")
		klog.Infof("taking a short nap to let the ESX settle down")
		time.Sleep(5 * time.Second)
		deleteDeadDevices(p.VSphereClient, host, hbaUIDs, hbaUIDsNamesMap)
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
		err = vsphere_offload.EnableSSHAccess(sshSetupCtx, p.VSphereClient, host, p.SSHPrivateKey, p.SSHPublicKey, finalScriptPath)
		if err != nil {
			return fmt.Errorf("failed to enable SSH access: %w", err)
		}

		// Get host IP
		hostIP, err := vsphere_offload.GetHostIPAddress(sshSetupCtx, host)
		if err != nil {
			return fmt.Errorf("failed to get host IP address: %w", err)
		}

		// Create SSH client with background context (no timeout for long-running operations)
		sshClient := vsphere_offload.NewSSHClient()
		err = sshClient.Connect(context.Background(), hostIP, "root", p.SSHPrivateKey)
		if err != nil {
			return fmt.Errorf("failed to connect via SSH: %w", err)
		}
		defer sshClient.Close()

		klog.V(2).Infof("SSH connection established with restricted commands")
		// Valdate the uploaded script version matches the embedded script version
		err = checkScriptVersion(sshClient, vmDisk.Datastore, vmkfstoolswrapper.Version, p.SSHPublicKey)
		if err != nil {
			return fmt.Errorf("script version check failed: %w", err)
		}

		executor = NewSSHTaskExecutor(sshClient)
	} else {
		executor = NewVIBTaskExecutor(p.VSphereClient)
	}

	// Use unified task execution
	return ExecuteCloneTask(context.Background(), executor, host, vmDisk.Datastore, vmDisk.Path(), targetLUN, progress, xcopyUsed)
}

// waitForDeviceStateOff waits for the device state to become "off" using exponential backoff
func waitForDeviceStateOff(client vmware.Client, host *object.HostSystem, deviceNAA string) error {
	backoff := wait.Backoff{
		Duration: 1 * time.Second,
		Factor:   2.0,
		Jitter:   0.1,
		Steps:    10, // Max retries
	}

	return wait.ExponentialBackoff(backoff, func() (bool, error) {
		result, err := client.RunEsxCommand(context.Background(), host, []string{"storage", "core", "device", "list", "-d", deviceNAA})
		if err != nil {
			klog.V(2).Infof("failed to check device %s state: %v", deviceNAA, err)
			return false, nil // Retry on error
		}

		if len(result) > 0 && result[0] != nil && len(result[0]["Status"]) > 0 {
			status := result[0]["Status"][0]
			klog.V(2).Infof("device %s status: %s", deviceNAA, status)
			if status == "off" {
				klog.Infof("device %s state is now off", deviceNAA)
				return true, nil // Success
			}
		}

		return false, nil // Retry
	})
}

// After mapping a volume the ESX needs a rescan to see the device. ESXs can opt-in to do it automatically
func rescan(ctx context.Context, client vmware.Client, host *object.HostSystem, targetLUN string) error {
	for i := 1; i <= rescanRetries; i++ {
		// Check if we should abort (lease was lost)
		if ctx.Err() != nil {
			return fmt.Errorf("rescan aborted (lease lost): %w", ctx.Err())
		}

		result, err := client.RunEsxCommand(context.Background(), host, []string{"storage", "core", "device", "list", "-d", targetLUN})
		if err == nil {
			status := ""
			if result != nil && result[0] != nil && len(result[0]["Status"]) > 0 {
				status = result[0]["Status"][0]
			}
			klog.Infof("found device %s with status %v", targetLUN, status)
			if status == "off" || status == "dead timeout" {
				klog.Infof("try to remove the device from the detached list (this can happen if restarting this pod or using the same volume)")
				_, err = client.RunEsxCommand(context.Background(), host, []string{"storage", "core", "device", "detached", "remove", "-d", targetLUN})
				continue
			}
			return nil
		} else {
			_, err = client.RunEsxCommand(
				context.Background(), host, []string{"storage", "core", "adapter", "rescan", "-t", "add", "-a", "1"})
			if err != nil {
				klog.Errorf("failed to rescan for adapters, attempt %d/%d due to: %s", i, rescanRetries, err)
			}

			// Sleep but respect context cancellation
			select {
			case <-time.After(rescanSleepInterval):
				// Continue to next iteration
			case <-ctx.Done():
				return fmt.Errorf("rescan aborted during retry sleep (lease lost): %w", ctx.Err())
			}
		}
	}

	// Check one more time before final attempt
	if ctx.Err() != nil {
		return fmt.Errorf("rescan aborted before final attempt (lease lost): %w", ctx.Err())
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

func deleteDeadDevices(client vmware.Client, host *object.HostSystem, hbaUIDs []string, hbaUIDsNamesMap map[string]string) error {
	failedDevices := []string{}
	for _, adapter := range hbaUIDs {
		adapterName, ok := hbaUIDsNamesMap[adapter]
		if !ok {
			adapterName = adapter
		}
		klog.Infof("deleting dead devices for adapter %s", adapterName)
		success := false
		for i := 0; i < rescanRetries; i++ {
			_, errClean := client.RunEsxCommand(
				context.Background(),
				host,
				[]string{"storage", "core", "adapter", "rescan", "-t", "delete", "-A", adapterName})
			if errClean == nil {
				klog.Infof("rescan to delete dead devices completed for adapter %s", adapter)
				success = true
				break // finsihed with current adapter, move to the next one
			}
			time.Sleep(rescanSleepInterval)
		}
		if !success {
			failedDevices = append(failedDevices, adapter)
		}
	}
	if len(failedDevices) > 0 {
		klog.Warningf("failed to delete dead devices for adapters %s", failedDevices)
	}
	return nil
}

func checkScriptVersion(sshClient vsphere_offload.SSHClient, datastore, embeddedVersion string, publicKey []byte) error {
	output, err := sshClient.ExecuteCommand(datastore, "--version")
	if err != nil {
		return fmt.Errorf("old script format detected (likely Python-based). Update script on datastore %s to version %s or newer: %w", datastore, embeddedVersion, err)
	}

	var resp XMLResponse
	if err := xml.Unmarshal([]byte(output), &resp); err != nil {
		return fmt.Errorf("failed to parse version response: %w", err)
	}

	var status, message string
	for _, f := range resp.Structure.Fields {
		switch f.Name {
		case "status":
			status = f.String
		case "message":
			message = f.String
		}
	}
	if status != "0" || message == "" {
		return fmt.Errorf("version command failed: status=%s, message=%s", status, message)
	}

	var versionInfo struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal([]byte(message), &versionInfo); err != nil {
		return fmt.Errorf("failed to parse version JSON: %w", err)
	}

	scriptVer, err := version.NewVersion(versionInfo.Version)
	if err != nil {
		return fmt.Errorf("invalid script version format %s: %w", versionInfo.Version, err)
	}

	embeddedVer, err := version.NewVersion(embeddedVersion)
	if err != nil {
		return fmt.Errorf("invalid embedded version format %s: %w", embeddedVersion, err)
	}

	if scriptVer.LessThan(embeddedVer) {
		publicKeyStr := string(publicKey)
		restrictedPublicKey := fmt.Sprintf(`command="%s",no-port-forwarding,no-agent-forwarding,no-X11-forwarding %s`,
			util.RestrictedSSHCommandTemplate, publicKeyStr)

		klog.Errorf("Version mismatch detected!")
		klog.Errorf("  - Just uploaded script version: %s", embeddedVersion)
		klog.Errorf("  - SSH returned version: %s", versionInfo.Version)
		klog.Errorf("")
		klog.Errorf("This indicates the SSH key is executing a different script file.")
		klog.Errorf("Most likely cause: You are using the old Python-based SSH key format")
		klog.Errorf("which executes a file with .py extension or UUID in the filename.")
		klog.Errorf("")
		klog.Errorf("The new shell-based format executes:")
		klog.Errorf("  /vmfs/volumes/%s/secure-vmkfstools-wrapper (no extension)", datastore)
		klog.Errorf("")
		klog.Errorf("To fix this issue:")
		klog.Errorf("1. SSH to the ESXi host")
		klog.Errorf("2. Edit /etc/ssh/keys-root/authorized_keys: vi /etc/ssh/keys-root/authorized_keys")
		klog.Errorf("3. Find the line containing the old Python wrapper")
		klog.Errorf("4. DELETE the line containing .py extension or UUID in filename")
		klog.Errorf("   Examples of old format to remove:")
		klog.Errorf("     - Lines ending with: secure-vmkfstools-wrapper.py")
		klog.Errorf("     - Lines ending with: secure-vmkfstools-wrapper-$UUID.py")
		klog.Errorf("5. Add the following NEW SSH key line:")
		klog.Errorf("")
		klog.Errorf("  %s", restrictedPublicKey)
		klog.Errorf("")
		klog.Errorf("6. Save and exit")
		klog.Errorf("7. Retry the operation")

		return fmt.Errorf("version mismatch: uploaded %s but SSH returned %s - old SSH key format detected",
			embeddedVersion, versionInfo.Version)
	}

	return nil
}
