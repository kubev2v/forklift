package populator

import (
	"context"
	"fmt"

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

func (p *RemoteEsxcliPopulator) Populate(sourceVMDKFile string, volumeHandle string, progress chan int, quit chan string) error {
	vmDisk, err := ParseVmdkPath(sourceVMDKFile)
	if err != nil {
		return err
	}

	klog.Infof("Starting to populate using remote esxcli vmkfstools, source vmdk %s target LUN %s", sourceVMDKFile, volumeHandle)
	lun, err := p.StorageApi.ResolveVolumeHandleToLUN(volumeHandle)
	if err != nil {
		return err
	}

	originalInitiatorGroups, err := p.StorageApi.CurrentMappedGroups(lun)
	if err != nil {
		return fmt.Errorf("failed to fetch the current initiator groups of the lun %s: %w", lun.Name, err)
	}

	targetLUN := fmt.Sprintf("/vmfs/devices/disks/naa.%s%x", lun.ProviderID, lun.SerialNumber)
	klog.Infof("resolved lun serial number %s with IQN %s to lun %s", lun.SerialNumber, lun.IQN, targetLUN)

	host, err := p.VSphereClient.GetEsxByVm(context.Background(), vmDisk.VMName)
	if err != nil {
		return err
	}
	klog.Infof("Working with ESXi %+v", host)

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

	err = p.StorageApi.EnsureClonnerIgroup(xcopyInitiatorGroup, esxIQN)
	if err != nil {
		return fmt.Errorf("failed to add the ESX IQN %s to the initiator group %w", esxIQN, err)
	}

	err = p.StorageApi.Map(xcopyInitiatorGroup, lun)
	if err != nil {
		return fmt.Errorf("failed to map lun %s to initiator group %s: %w", lun, xcopyInitiatorGroup, err)
	}
	defer func() {
		p.StorageApi.UnMap(xcopyInitiatorGroup, lun)
		for _, group := range originalInitiatorGroups {
			p.StorageApi.Map(group, lun)
		}

	}()

	_, err = p.VSphereClient.RunEsxCommand(context.Background(), host, []string{"storage", "core", "adapter", "rescan", "-a", "1"})
	if err != nil {
		return err
	}
	naa := fmt.Sprintf("naa.%s%x", lun.ProviderID, lun.SerialNumber)
	_, err = p.VSphereClient.RunEsxCommand(context.Background(), host, []string{"storage", "core", "device", "list", "-d", naa})
	if err != nil {
		return fmt.Errorf("failed to locate the target LUN %s. Check the LUN details and the host mapping response: %s", naa, err)
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
		quit <- response
	}()
	return nil
}
