package populator

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"slices"
	"time"

	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/vmware"
	"k8s.io/klog/v2"
)

const xcopyInitiatorGroup = "xcopy-esxs"

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

func (p *RemoteEsxcliPopulator) Populate(sourceVMDKFile string, volumeHandle string, progress chan int, quit chan error) (err error) {
	// isn't it better to not call close the channel from the caller?
	defer func() {
		if err != nil {
			quit <- err
		}
	}()
	vmDisk, err := ParseVmdkPath(sourceVMDKFile)
	if err != nil {
		return err
	}
	klog.Infof("Starting to populate using remote esxcli vmkfstools, source vmdk %s target LUN %s", sourceVMDKFile, volumeHandle)
	host, err := p.VSphereClient.GetEsxByVm(context.Background(), vmDisk.VMName)
	if err != nil {
		return err
	}
	klog.Infof("Got ESXI host: %s", host)

	// for iSCSI add the host to the group using IQN. Is there something else for FC?
	r, err := p.VSphereClient.RunEsxCommand(context.Background(), host, []string{"iscsi", "adapter", "list"})
	if err != nil {
		return err
	}
	esxIQN := ""
	for _, a := range r {
		// get the first adapter iqn
		iqnValue, ok := a["UID"]
		if !ok || len(iqnValue) == 0 {
			return fmt.Errorf("failed to extract the IQN from the adapter item%s", a)
		}
		esxIQN = iqnValue[0]
		klog.Infof("iSCSI adapter IQN %s", esxIQN)
	}

	mappingContext, err := p.StorageApi.EnsureClonnerIgroup(xcopyInitiatorGroup, esxIQN)
	if err != nil {
		return fmt.Errorf("failed to add the ESX IQN %s to the initiator group %w", esxIQN, err)
	}

	lun, err := p.StorageApi.ResolveVolumeHandleToLUN(volumeHandle)
	if err != nil {
		return err
	}

	lun, err = p.StorageApi.Map(xcopyInitiatorGroup, lun, mappingContext)
	if err != nil {
		return fmt.Errorf("failed to map lun %s to initiator group %s: %w", lun, xcopyInitiatorGroup, err)
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
	esxNaa := fmt.Sprintf("naa.%s", lun.NAA)

	targetLUN := fmt.Sprintf("/vmfs/devices/disks/%s", esxNaa)
	klog.Infof("resolved lun with IQN %s to lun %s", lun.IQN, targetLUN)

	retries := 5
	for i := 1; i < retries; i++ {
		_, err = p.VSphereClient.RunEsxCommand(context.Background(), host, []string{"storage", "core", "device", "list", "-d", esxNaa})
		if err == nil {
			klog.Infof("found device %s", esxNaa)
			break
		} else {
			_, err = p.VSphereClient.RunEsxCommand(context.Background(), host, []string{"storage", "core", "adapter", "rescan", "-t", "add", "-a", "1"})
			if err != nil {
				klog.Errorf("failed to rescan for adapters, atepmt %d/%d due to: %s", i, retries, err)
				i, err := rand.Int(rand.Reader, big.NewInt(10))
				if err != nil {
					return fmt.Errorf("failed to randomize a sleep interval: %w", err)
				}
				time.Sleep(time.Duration(i.Int64()) * time.Second)
			}
		}
	}
	if err != nil {
		return fmt.Errorf("failed to find the device %s after scanning: %w", esxNaa, err)
	}

	defer func() {
		p.StorageApi.UnMap(xcopyInitiatorGroup, lun, mappingContext)
		// map the LUN back to the original OCP worker
		for _, group := range originalInitiatorGroups {
			p.StorageApi.Map(group, lun, mappingContext)
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
	klog.Info("verifying copy succeeded")
	r, err = p.VSphereClient.RunEsxCommand(context.Background(), host, []string{"md5sum", esxNaa})
	if err != nil {
		klog.Infof("error during checking naa, response from esxcli %+v", r)
		return err
	}
	fmt.Println("md5sum >>>>>>>>>> %w", r)
	r, err = p.VSphereClient.RunEsxCommand(context.Background(), host, []string{"md5sum", vmDisk.Path()})
	if err != nil {
		klog.Infof("error during checking vmdk file, response from esxcli %+v", r)
		return err
	}
	fmt.Println("md5sum >>>>>>>>>> %w", r)
	go func() {
		// TODO need to process the vmkfstools stderr(probably) and to write the
		// progress to a file, and then continuously read and report on the channel
		progress <- 100
		quit <- nil
	}()
	return nil
}
