package populator

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/vmware"
	"github.com/vmware/govmomi/object"
	"k8s.io/klog/v2"
)

const xcopyInitiatorGroup = "xcopy-esxs"
const taskPollingInterval = 5 * time.Second

var progressPattern = regexp.MustCompile(`\s(\d+)\%`)

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
}

func NewWithRemoteEsxcli(storageApi StorageApi, vsphereHostname, vsphereUsername, vspherePassword string) (Populator, error) {
	c, err := vmware.NewClient(vsphereHostname, vsphereUsername, vspherePassword)
	if err != nil {
		return nil, fmt.Errorf("failed to create vmware client: %w", err)
	}
	return &RemoteEsxcliPopulator{
		VSphereClient: c,
		StorageApi:    storageApi,
		UseSSHMethod:  false, // VIB method
	}, nil
}

func NewWithRemoteEsxcliSSH(storageApi StorageApi, vsphereHostname, vsphereUsername, vspherePassword string, sshPrivateKey, sshPublicKey []byte) (Populator, error) {
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
	}, nil
}

func (p *RemoteEsxcliPopulator) Populate(vmId string, sourceVMDKFile string, pv PersistentVolume, progress chan<- uint, quit chan error) (errFinal error) {
	// isn't it better to not call close the channel from the caller?
	defer func() {
		r := recover()
		if r != nil {
			klog.Infof("recovered %v", r)
		}
		quit <- errFinal
	}()
	vmDisk, err := ParseVmdkPath(sourceVMDKFile)
	if err != nil {
		return err
	}

	var cloneMethodStr string
	if p.UseSSHMethod {
		cloneMethodStr = "SSH"
	} else {
		cloneMethodStr = "VIB"
	}

	klog.Infof(
		"Starting populate via remote esxcli vmkfstools (%s), source vmdk=%s, pv=%v",
		cloneMethodStr,
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
		// powerflex handling - scini is the powerflex kernel module and is not
		// using any iqn/wwn to identity the host. Instead extract the SdcGuid
		// as the possible clonner identifier
		if slices.Contains(driver, "scini") {
			sciModule, err := p.VSphereClient.RunEsxCommand(context.Background(), host, []string{"system", "module", "parameters", "list", "-m", "scini"})
			if err != nil {
				// TODO skip, but print the error. Perhaps this handling is better suited per-vendor?
				klog.Infof("failed to fetch the scini module parameters %s: ", err)
				continue
			}
			for _, moduleFields := range sciModule {

				if slices.Contains(moduleFields["Name"], "IoctlIniGuidStr") {
					klog.Infof("scini guid %v", moduleFields["Value"])
					for _, s := range moduleFields["Value"] {
						hbaUIDs = append(hbaUIDs, strings.ToUpper(s))
					}
					klog.Infof("hbas %+v", hbaUIDs)
				}
			}
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

	defer func() {
		if !slices.Contains(originalInitiatorGroups, xcopyInitiatorGroup) {
			p.StorageApi.UnMap(xcopyInitiatorGroup, lun, mappingContext)
		}
	}()

	lun, err = p.StorageApi.Map(xcopyInitiatorGroup, lun, mappingContext)
	if err != nil {
		return fmt.Errorf("failed to map lun %s to initiator group %s: %w", lun, xcopyInitiatorGroup, err)
	}

	targetLUN := fmt.Sprintf("/vmfs/devices/disks/%s", lun.NAA)
	klog.Infof("resolved lun with IQN %s to lun %s", lun.IQN, targetLUN)

	retries := 5
	for i := 1; i <= retries; i++ {
		_, err = p.VSphereClient.RunEsxCommand(context.Background(), host, []string{"storage", "core", "device", "list", "-d", targetLUN})
		if err == nil {
			klog.Infof("found device %s", targetLUN)
			break
		} else {
			_, err = p.VSphereClient.RunEsxCommand(
				context.Background(), host, []string{"storage", "core", "adapter", "rescan", "-t", "add", "-a", "1"})
			if err != nil {
				klog.Errorf("failed to rescan for adapters, attempt %d/%d due to: %s", i, retries, err)
				time.Sleep(5 * time.Second)
			}
		}
	}
	if err != nil {
		return fmt.Errorf("failed to find the device %s after scanning: %w", targetLUN, err)
	}

	defer func() {
		klog.Infof("cleaning up - unmap and rescan to clean dead devices")
		p.StorageApi.UnMap(xcopyInitiatorGroup, lun, mappingContext)
		// map the LUN back to the original OCP worker
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

	// Execute the clone using the appropriate method
	if p.UseSSHMethod {
		return p.executeSSHClone(host, vmDisk, targetLUN, progress)
	} else {
		return p.executeVIBClone(host, vmDisk, targetLUN, progress)
	}
}

// executeVIBClone performs the clone using the original VIB method
func (p *RemoteEsxcliPopulator) executeVIBClone(host *object.HostSystem, vmDisk VMDisk, targetLUN string, progress chan<- uint) error {
	r, err := p.VSphereClient.RunEsxCommand(context.Background(), host, []string{"vmkfstools", "clone", "-s", vmDisk.Path(), "-t", targetLUN})
	if err != nil {
		klog.Infof("error during copy, response from esxcli %+v", r)
		return err
	}

	response := ""
	klog.Info("respose from esxcli ", r)
	for _, l := range r {
		response += l.Value("message")
	}

	v := vmkfstoolsClone{}
	err = json.Unmarshal([]byte(response), &v)
	if err != nil {
		return err
	}

	if v.TaskId != "" {
		defer func() {
			klog.Info("cleaning up task artifacts")
			r, errClean := p.VSphereClient.RunEsxCommand(context.Background(),
				host, []string{"vmkfstools", "taskClean", "-i", v.TaskId})
			if errClean != nil {
				klog.Errorf("failed cleaning up task artifacts %v", r)
			}
		}()
	}
	for {
		r, err = p.VSphereClient.RunEsxCommand(context.Background(),
			host, []string{"vmkfstools", "taskGet", "-i", v.TaskId})
		if err != nil {
			return err
		}
		response := ""
		klog.Info("respose from esxcli ", r)
		for _, l := range r {
			response += l.Value("message")
		}
		v := vmkfstoolsTask{}
		err = json.Unmarshal([]byte(response), &v)
		if err != nil {
			klog.Errorf("failed to unmarshal response from esxcli %+v", r)
			return err
		}

		klog.Infof("respose from esxcli %+v", v)

		// exmple output - Clone: 20% done.
		match := progressPattern.FindStringSubmatch(v.LastLine)
		if len(match) > 1 {
			i, _ := strconv.Atoi(match[1])
			progress <- uint(i)
		}

		if v.ExitCode != "" {
			if v.ExitCode == "0" {
				err = nil
			} else {
				err = fmt.Errorf("failed with exit code %s with stderr: %s", v.ExitCode, v.Stderr)
			}
			return err
		}

		time.Sleep(taskPollingInterval)
	}
}

// executeSSHClone performs the clone using the SSH method
func (p *RemoteEsxcliPopulator) executeSSHClone(host *object.HostSystem, vmDisk VMDisk, targetLUN string, progress chan<- uint) error {
	// Create a context with timeout to bound SSH enable/connect and script staging operations
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Setup secure script
	finalScriptPath, err := ensureSecureScript(ctx, p.VSphereClient, host, vmDisk.Datastore, SecureScriptVersion)
	if err != nil {
		return fmt.Errorf("failed to ensure secure script: %w", err)
	}
	klog.V(2).Infof("Secure script ready at path: %s", finalScriptPath)

	// Enable SSH access
	err = vmware.EnableSSHAccess(ctx, p.VSphereClient, host, p.SSHPrivateKey, p.SSHPublicKey, finalScriptPath)
	if err != nil {
		return fmt.Errorf("failed to enable SSH access: %w", err)
	}

	// Get host IP
	hostIP, err := vmware.GetHostIPAddress(ctx, host)
	if err != nil {
		return fmt.Errorf("failed to get host IP address: %w", err)
	}

	// Create SSH client
	sshClient := vmware.NewSSHClient()
	err = sshClient.Connect(ctx, hostIP, "root", p.SSHPrivateKey)
	if err != nil {
		return fmt.Errorf("failed to connect via SSH: %w", err)
	}
	defer sshClient.Close()

	klog.V(2).Infof("SSH connection established with restricted commands")

	// Start the clone task
	task, err := sshClient.StartVmkfstoolsClone(vmDisk.Path(), targetLUN)
	if err != nil {
		return fmt.Errorf("failed to start vmkfstools clone: %w", err)
	}

	klog.Infof("Started vmkfstools clone task %s", task.TaskId)

	if task.TaskId != "" {
		defer func() {
			err := sshClient.CleanupTask(task.TaskId)
			if err != nil {
				klog.Errorf("Failed cleaning up task artifacts: %v", err)
			}
		}()
	}

	// Poll for task completion
	for {
		taskStatus, err := sshClient.GetTaskStatus(task.TaskId)
		if err != nil {
			return fmt.Errorf("failed to get task status: %w", err)
		}

		klog.V(2).Infof("Task status: %+v", taskStatus)

		if progressValue, hasProgress := vmware.ParseProgress(taskStatus.LastLine); hasProgress {
			progress <- progressValue
		}

		if taskStatus.ExitCode != "" {
			if taskStatus.ExitCode == "0" {
				klog.Infof("vmkfstools clone completed successfully")
				return nil
			} else {
				return fmt.Errorf("vmkfstools clone failed with exit code %s, stderr: %s", taskStatus.ExitCode, taskStatus.Stderr)
			}
		}

		time.Sleep(taskPollingInterval)
	}
}
