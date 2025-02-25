package populator

import (
	"context"
	"fmt"
	"math/rand"
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
	klog.Infof("Got ESXI host: %s", host)
	klog.Infof("Got ESXI name: %s", host.Name())
	if err != nil {
		return err
	}

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

	m, err := p.StorageApi.EnsureClonnerIgroup(xcopyInitiatorGroup, esxIQN)
	if err != nil {
		return fmt.Errorf("failed to add the ESX IQN %s to the initiator group %w", esxIQN, err)
	}

	lun, err := p.StorageApi.ResolveVolumeHandleToLUN(volumeHandle)
	if err != nil {
		return err
	}

	lun, err = p.StorageApi.Map(xcopyInitiatorGroup, lun, m)
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
			p.StorageApi.UnMap(xcopyInitiatorGroup, lun, m)
		}
	}()
	esxNaa := fmt.Sprintf("naa.%s", lun.NAA)

	targetLUN := fmt.Sprintf("/vmfs/devices/disks/%s", esxNaa)
	klog.Infof("resolved lun with IQN %s to lun %s", lun.IQN, targetLUN)

	retries := 3
	for i := 3; i > 0; i-- {
		_, err = p.VSphereClient.RunEsxCommand(context.Background(), host, []string{"storage", "core", "adapter", "rescan", "-a", "1"})
		if err != nil {
			klog.Errorf("failed to rescan adapters, probably in progress. Rerty %d/%d", i, retries)
			time.Sleep(time.Duration(rand.Intn(10)))
		} else {
			break
		}
	}
	if err != nil {
		return err
	}

	_, err = p.VSphereClient.RunEsxCommand(context.Background(), host, []string{"storage", "core", "device", "list", "-d", esxNaa})
	if err != nil {
		return fmt.Errorf("failed to locate the target LUN %s. Check the LUN details and the host mapping response: %s", esxNaa, err)
	}

	r, err = p.VSphereClient.RunEsxCommand(context.Background(), host, []string{"vmkfstools", "clone", "-s", vmDisk.Path(), "-t", targetLUN})
	if err != nil {
		klog.Infof("error response from esxcli %+v", r)
		return err
	}

	response := ""
	klog.Info("respose from esxcli ", r)

	for _, l := range r {
		response += l.Value("message")
	}

	go func() {
		// TODO need to process the vmkfstools stderr(probably) and to write the
		// progress to a file, and then continuously read and report on the channel
		progress <- 100
		quit <- nil
	}()
	return nil
}
