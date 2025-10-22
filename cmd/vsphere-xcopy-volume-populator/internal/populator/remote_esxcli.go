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
}

func NewWithRemoteEsxcli(storageApi StorageApi, vsphereHostname, vsphereUsername, vspherePassword string) (Populator, error) {
	c, err := vmware.NewClient(vsphereHostname, vsphereUsername, vspherePassword)
	if err != nil {
		return nil, fmt.Errorf("failed to create vmware client: %w", err)
	}
	return &RemoteEsxcliPopulator{
		VSphereClient: c,
		StorageApi:    storageApi,
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
	klog.Infof(
		"Starting to populate using remote esxcli vmkfstools, source vmdk %s target LUN %s",
		sourceVMDKFile,
		pv)
	host, err := p.VSphereClient.GetEsxByVm(context.Background(), vmId)
	if err != nil {
		return err
	}
	klog.Infof("Got ESXi host: %s", host)

	err = ensureVib(p.VSphereClient, host, vmDisk.Datastore, VibVersion)
	if err != nil {
		return fmt.Errorf("failed to ensure VIB is installed: %w", err)
	}
	// for iSCSI add the host to the group using IQN. Is there something else for FC?
	r, err := p.VSphereClient.RunEsxCommand(context.Background(), host, []string{"storage", "core", "adapter", "list"})
	if err != nil {
		return err
	}
	uniqueUIDs := make(map[string]bool)
	hbaUIDs := []string{}

	// Print the adapter information for debugging
	for i, val := range r {
		klog.Infof("Adapter [%d]: %+v", i, val)
		for key, field := range val {
			klog.Infof("  %s: %v", key, field)
		}
	}

	for _, a := range r {
		driver, hasDriver := a["Driver"]
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

	var mappingContext map[string]interface{}
	var maperr error
	// For PowerMax - add esxi host name
	if pmx, ok := p.StorageApi.(PowerMaxApi); ok {
		klog.Infof("ESX host name %s: ", host.Name())
		esxname, err := host.ObjectName(context.Background())
		if err != nil {
			klog.Infof("object name failed")
		}
		klog.Infof("ESX host name %s: ", esxname)
		esxiHostName := esxname
		mappingContext, maperr = pmx.EnsureClonnerIgroupWithHost(xcopyInitiatorGroup, hbaUIDs, esxiHostName)
	} else {
		mappingContext, maperr = p.StorageApi.EnsureClonnerIgroup(xcopyInitiatorGroup, hbaUIDs)
	}

	if maperr != nil {
		return fmt.Errorf("failed to add the ESX HBA UID %s to the initiator group %w", hbaUIDs, err)
	}

	lun, err := p.StorageApi.ResolvePVToLUN(pv)
	if err != nil {
		return err
	}

	originalInitiatorGroups, err := p.StorageApi.CurrentMappedGroups(lun, nil)
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

	fmt.Println("Sleeping...")
	time.Sleep(60 * time.Second) // Pauses execution for 30 seconds
	fmt.Println("Resuming after 60 seconds.")

	//esxNaa := fmt.Sprintf("naa.%s", lun.NAA)
	esxNaa := fmt.Sprintf("%s", lun.NAA)

	targetLUN := fmt.Sprintf("/vmfs/devices/disks/%s", esxNaa)
	klog.Infof("resolved lun with IQN %s to lun %s", lun.IQN, targetLUN)

	retries := 5
	for i := 1; i < retries; i++ {
		_, err = p.VSphereClient.RunEsxCommand(context.Background(), host, []string{"storage", "core", "device", "list", "-d", esxNaa})
		if err == nil {
			klog.Infof("found device %s", esxNaa)
			break
		} else {
			_, err = p.VSphereClient.RunEsxCommand(
				context.Background(), host, []string{"storage", "core", "adapter", "rescan", "-t", "add", "-a", "1"})
			if err != nil {
				klog.Errorf("failed to rescan for adapters, atepmt %d/%d due to: %s", i, retries, err)
				time.Sleep(5 * time.Second)
			}
		}
	}
	if err != nil {
		return fmt.Errorf("failed to find the device %s after scanning: %w", esxNaa, err)
	}

	defer func() {
		klog.Infof("cleaning up - unmap and rescan to clean dead devices")
		p.StorageApi.UnMap(xcopyInitiatorGroup, lun, mappingContext)
		// map the LUN back to the original OCP worker
		for _, group := range originalInitiatorGroups {
			p.StorageApi.Map(group, lun, mappingContext)
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

	r, err = p.VSphereClient.RunEsxCommand(context.Background(), host, []string{"vmkfstools", "clone", "-s", vmDisk.Path(), "-t", targetLUN})
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
