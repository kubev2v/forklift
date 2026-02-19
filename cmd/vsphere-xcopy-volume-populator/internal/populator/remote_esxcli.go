package populator

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"slices"
	"strings"
	"time"

	hversion "github.com/hashicorp/go-version"
	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/version"
	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/vmware"
	vmkfstoolswrapper "github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/vmkfstools-wrapper"
	"github.com/kubev2v/forklift/pkg/lib/util"
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
	log := klog.Background().WithName("copy-offload").WithName("xcopy")
	setupLog := log.WithName("setup")
	mapLog := log.WithName("map-volume")
	rescanLog := log.WithName("rescan")
	cloneLog := log.WithName("clone")
	cleanupLog := log.WithName("cleanup")

	defer func() {
		r := recover()
		if r != nil {
			log.Info("recovered from panic", "panic", r)
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
	setupLog.Info("VMDK/Xcopy populate started", "method", cloneMethod, "source", sourceVMDKFile, "target", pv.Name)

	setupCtx := klog.NewContext(context.Background(), setupLog)
	host, err := p.VSphereClient.GetEsxByVm(setupCtx, vmId)
	if err != nil {
		return err
	}
	setupLog.Info("ESXi host", "host", host.String())

	hostID := strings.ReplaceAll(strings.ToLower(host.String()), ":", "-")
	xcopyInitiatorGroup := fmt.Sprintf("xcopy-%s", hostID)
	setupLog.Info("initiator group", "group", xcopyInitiatorGroup)

	// Only ensure VIB if using VIB method
	if !p.UseSSHMethod {
		err = ensureVib(setupCtx, p.VSphereClient, host, vmDisk.Datastore, version.VibVersion)
		if err != nil {
			return fmt.Errorf("failed to ensure VIB is installed: %w", err)
		}
	}

	// for iSCSI add the host to the group using IQN. Is there something else for FC?
	r, err := p.VSphereClient.RunEsxCommand(setupCtx, host, []string{"storage", "core", "adapter", "list"})
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
		setupLog.V(2).Info("scini required for storage api")
		sciModule, err := p.VSphereClient.RunEsxCommand(setupCtx, host, []string{"system", "module", "parameters", "list", "-m", "scini"})
		if err != nil {
			setupLog.Info("failed to fetch scini module parameters", "err", err)
			return err
		}
		for _, moduleFields := range sciModule {

			if slices.Contains(moduleFields["Name"], "IoctlIniGuidStr") {
				setupLog.V(2).Info("scini guid", "value", moduleFields["Value"])
				for _, s := range moduleFields["Value"] {
					hbaUIDs = append(hbaUIDs, strings.ToUpper(s))
				}
				setupLog.Info("scini HBAs found", "hbas", hbaUIDs)
			}
		}
	}

	if !isSciniRequired {
		setupLog.V(2).Info("scini not required for storage api")
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
					setupLog.V(2).Info("storage adapter", "uid", id, "driver", drv)
				}
			}
		}
		setupLog.Info("HBA UIDs found", "count", len(hbaUIDs), "uids", hbaUIDs)
	}

	if len(hbaUIDs) == 0 {
		setupLog.Info("no valid HBA UIDs found", "host", host.String())
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
	setupLog.V(2).Info("current initiator groups for LUN", "lun", lun.IQN, "groups", originalInitiatorGroups)

	if isSciniRequired {
		sdcId, ok := mappingContext["sdcId"]
		if !ok {
			setupLog.Info("sdcId required but not in mappingContext")
			return fmt.Errorf("sdcId is required but not found in mappingContext")
		}
		xcopyInitiatorGroup = sdcId.(string)
		setupLog.V(2).Info("sdcId from mappingContext", "sdcId", sdcId)
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
					setupLog.Info("partial cleanup: failed to unmap", "err", errUnmap)
				}
			} else {
				setupLog.V(2).Info("skipping partial cleanup unmap (LUN not resolved)")
			}
		}
	}()

	mapLog.Info("mapping volume to initiator group", "initiator_group", xcopyInitiatorGroup, "lun", lun.Name)
	lun, err = p.StorageApi.Map(xcopyInitiatorGroup, lun, mappingContext)
	if err != nil {
		return fmt.Errorf("failed to map lun %s to initiator group %s: %w", lun, xcopyInitiatorGroup, err)
	}

	targetLUN := fmt.Sprintf("/vmfs/devices/disks/%s", lun.NAA)
	mapLog.Info("volume mapped to initiator group", "initiator_group", xcopyInitiatorGroup, "device", targetLUN)

	leaseHostID := strings.ReplaceAll(strings.ToLower(host.String()), ":", "-")
	rescanLog.Info("rescanning host for device", "device", lun.NAA)
	err = hostLocker.WithLock(context.Background(), leaseHostID,
		func(ctx context.Context) error {
			return rescan(ctx, p.VSphereClient, host, lun.NAA)
		},
	)
	if err != nil {
		return fmt.Errorf("failed to find the device %s after scanning: %w", targetLUN, err)
	}
	rescanLog.Info("device visible on ESXi", "device", targetLUN)

	defer func() {
		cleanupLog.Info("cleanup started: unmap and remove device")
		fullCleanUpAttempted = true
		cleanupCtx := klog.NewContext(context.Background(), cleanupLog)
		if mappingContext != nil {
			mappingContext["UnmapAllSdc"] = true
			mappingContext[CleanupXcopyInitiatorGroup] = true
		}

		// set device state to off and prevents any i/o to it
		_, err = p.VSphereClient.RunEsxCommand(cleanupCtx, host, []string{"storage", "core", "device", "set", "--state", "off", "-d", lun.NAA})
		if err != nil {
			cleanupLog.Info("failed to set device state off", "device", lun.Name, "err", err)
		} else {
			err = waitForDeviceStateOff(cleanupCtx, p.VSphereClient, host, lun.NAA)
			if err != nil {
				cleanupLog.Info("timeout waiting for device off", "device", lun.Name, "err", err)
			}
		}
		_, err = p.VSphereClient.RunEsxCommand(cleanupCtx, host, []string{"storage", "core", "device", "detached", "remove", "-d", lun.NAA})
		if err != nil {
			cleanupLog.Info("failed to remove device from detached list", "device", lun.Name, "err", err)
		}
		errUnmap := p.StorageApi.UnMap(xcopyInitiatorGroup, lun, mappingContext)
		if errUnmap != nil {
			cleanupLog.Info("failed to unmap during cleanup", "lun", lun.Name, "err", errUnmap)
		}

		cleanupLog.V(2).Info("mapping volume back to original initiator groups", "groups", originalInitiatorGroups)
		for _, group := range originalInitiatorGroups {
			_, errMap := p.StorageApi.Map(group, lun, mappingContext)
			if errMap != nil {
				cleanupLog.Info("failed to map volume back to original holder", "group", group, "err", errMap)
			}
		}
		cleanupLog.V(2).Info("deleting dead devices after short delay")
		time.Sleep(5 * time.Second)
		deleteDeadDevices(cleanupCtx, p.VSphereClient, host, hbaUIDs, hbaUIDsNamesMap)
		cleanupLog.Info("cleanup finished")
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
		setupLog.V(2).Info("secure script ready", "path", finalScriptPath)

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

		// Create SSH client; pass setup context so SSH logs show under setup.ssh
		sshClient := vmware.NewSSHClient()
		err = sshClient.Connect(sshSetupCtx, hostIP, "root", p.SSHPrivateKey)
		if err != nil {
			return fmt.Errorf("failed to connect via SSH: %w", err)
		}
		defer sshClient.Close()

		setupLog.V(2).Info("SSH connection established with restricted commands")
		// Validate the uploaded script version matches the embedded script version
		scriptVersionCtx := klog.NewContext(sshSetupCtx, setupLog)
		err = checkScriptVersion(scriptVersionCtx, sshClient, vmDisk.Datastore, vmkfstoolswrapper.Version, p.SSHPublicKey)
		if err != nil {
			return fmt.Errorf("script version check failed: %w", err)
		}

		executor = NewSSHTaskExecutor(sshClient)
	} else {
		executor = NewVIBTaskExecutor(p.VSphereClient)
	}

	// Use unified task execution (clone context so all clone/SSH logs show under clone)
	cloneCtx := klog.NewContext(context.Background(), cloneLog)
	return ExecuteCloneTask(cloneCtx, executor, host, vmDisk.Datastore, vmDisk.Path(), targetLUN, progress, xcopyUsed)
}

// waitForDeviceStateOff waits for the device state to become "off" using exponential backoff
func waitForDeviceStateOff(ctx context.Context, client vmware.Client, host *object.HostSystem, deviceNAA string) error {
	backoff := wait.Backoff{
		Duration: 1 * time.Second,
		Factor:   2.0,
		Jitter:   0.1,
		Steps:    10, // Max retries
	}

	log := klog.FromContext(ctx)
	return wait.ExponentialBackoff(backoff, func() (bool, error) {
		result, err := client.RunEsxCommand(ctx, host, []string{"storage", "core", "device", "list", "-d", deviceNAA})
		if err != nil {
			log.V(2).Info("device state check failed", "device", deviceNAA, "err", err)
			return false, nil
		}

		if len(result) > 0 && result[0] != nil && len(result[0]["Status"]) > 0 {
			status := result[0]["Status"][0]
			log.V(2).Info("device status", "device", deviceNAA, "status", status)
			if status == "off" {
				log.V(2).Info("device state is off", "device", deviceNAA)
				return true, nil
			}
		}

		return false, nil
	})
}

// After mapping a volume the ESX needs a rescan to see the device. ESXs can opt-in to do it automatically
func rescan(ctx context.Context, client vmware.Client, host *object.HostSystem, targetLUN string) error {
	log := klog.Background().WithName("copy-offload").WithName("xcopy").WithName("rescan")
	ctx = klog.NewContext(ctx, log)
	for i := 1; i <= rescanRetries; i++ {
		if ctx.Err() != nil {
			return fmt.Errorf("rescan aborted (lease lost): %w", ctx.Err())
		}

		result, err := client.RunEsxCommand(ctx, host, []string{"storage", "core", "device", "list", "-d", targetLUN})
		if err == nil {
			status := ""
			if result != nil && result[0] != nil && len(result[0]["Status"]) > 0 {
				status = result[0]["Status"][0]
			}
			log.V(2).Info("device list", "device", targetLUN, "status", status)
			if status == "off" || status == "dead timeout" {
				log.V(2).Info("removing device from detached list (restart or same volume)")
				_, _ = client.RunEsxCommand(ctx, host, []string{"storage", "core", "device", "detached", "remove", "-d", targetLUN})
				continue
			}
			return nil
		}
		_, err = client.RunEsxCommand(ctx, host, []string{"storage", "core", "adapter", "rescan", "-t", "add", "-a", "1"})
		if err != nil {
			log.Info("rescan attempt failed", "attempt", i, "max", rescanRetries, "err", err)
		}

		select {
		case <-time.After(rescanSleepInterval):
		case <-ctx.Done():
			return fmt.Errorf("rescan aborted during retry sleep (lease lost): %w", ctx.Err())
		}
	}

	if ctx.Err() != nil {
		return fmt.Errorf("rescan aborted before final attempt (lease lost): %w", ctx.Err())
	}

	_, err := client.RunEsxCommand(ctx, host, []string{"storage", "core", "device", "list", "-d", targetLUN})
	if err == nil {
		log.Info("found device", "device", targetLUN)
		return nil
	}
	return fmt.Errorf("failed to find device %s: %w", targetLUN, err)
}

func deleteDeadDevices(ctx context.Context, client vmware.Client, host *object.HostSystem, hbaUIDs []string, hbaUIDsNamesMap map[string]string) error {
	log := klog.FromContext(ctx)
	failedDevices := []string{}
	for _, adapter := range hbaUIDs {
		adapterName, ok := hbaUIDsNamesMap[adapter]
		if !ok {
			adapterName = adapter
		}
		log.V(2).Info("deleting dead devices for adapter", "adapter", adapterName)
		success := false
		for i := 0; i < rescanRetries; i++ {
			_, errClean := client.RunEsxCommand(ctx, host, []string{"storage", "core", "adapter", "rescan", "-t", "delete", "-A", adapterName})
			if errClean == nil {
				success = true
				break
			}
			time.Sleep(rescanSleepInterval)
		}
		if !success {
			failedDevices = append(failedDevices, adapter)
		}
	}
	if len(failedDevices) > 0 {
		log.V(0).Info("failed to delete dead devices for some adapters", "adapters", failedDevices, "severity", "warning")
	}
	return nil
}

func checkScriptVersion(ctx context.Context, sshClient vmware.SSHClient, datastore, embeddedVersion string, publicKey []byte) error {
	output, err := sshClient.ExecuteCommand(ctx, datastore, "--version")
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

	scriptVer, err := hversion.NewVersion(versionInfo.Version)
	if err != nil {
		return fmt.Errorf("invalid script version format %s: %w", versionInfo.Version, err)
	}

	embeddedVer, err := hversion.NewVersion(embeddedVersion)
	if err != nil {
		return fmt.Errorf("invalid embedded version format %s: %w", embeddedVersion, err)
	}

	if scriptVer.LessThan(embeddedVer) {
		publicKeyStr := string(publicKey)
		restrictedPublicKey := fmt.Sprintf(`command="%s",no-port-forwarding,no-agent-forwarding,no-X11-forwarding %s`,
			util.RestrictedSSHCommandTemplate, publicKeyStr)

		instructions := fmt.Sprintf(`Version mismatch detected!
  - Just uploaded script version: %s
  - SSH returned version: %s

This indicates the SSH key is executing a different script file.
Most likely cause: You are using the old Python-based SSH key format
which executes a file with .py extension or UUID in the filename.

The new shell-based format executes:
  /vmfs/volumes/%s/secure-vmkfstools-wrapper (no extension)

To fix this issue:
1. SSH to the ESXi host
2. Edit /etc/ssh/keys-root/authorized_keys: vi /etc/ssh/keys-root/authorized_keys
3. Find the line containing the old Python wrapper
4. DELETE the line containing .py extension or UUID in filename
   Examples of old format to remove:
     - Lines ending with: secure-vmkfstools-wrapper.py
     - Lines ending with: secure-vmkfstools-wrapper-$UUID.py
5. Add the following NEW SSH key line:

  %s

6. Save and exit
7. Retry the operation`, embeddedVersion, versionInfo.Version, datastore, restrictedPublicKey)
		setupLog := klog.Background().WithName("copy-offload").WithName("xcopy").WithName("setup")
		setupLog.Error(fmt.Errorf("version mismatch: uploaded %s, SSH returned %s", embeddedVersion, versionInfo.Version), "script version mismatch", "instructions", instructions)

		return fmt.Errorf("version mismatch: uploaded %s but SSH returned %s - old SSH key format detected",
			embeddedVersion, versionInfo.Version)
	}

	return nil
}
