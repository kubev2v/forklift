package powerflex

import (
	"fmt"
	"slices"
	"time"

	"github.com/dell/goscaleio"
	siotypes "github.com/dell/goscaleio/types/v1"
	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/populator"
	"k8s.io/klog/v2"
)

const (
	SYSTEM_ID_ENV_KEY        = "POWERFLEX_SYSTEM_ID"
	sdcIDContextKey   string = "sdcId"
)

type PowerflexClonner struct {
	Client   *goscaleio.Client
	systemId string
}

// CurrentMappedGroups implements populator.StorageApi.
func (p *PowerflexClonner) CurrentMappedGroups(targetLUN populator.LUN, mappingContext populator.MappingContext) ([]string, error) {
	klog.Infof("getting current mapping to volume %+v", targetLUN)
	klog.Infof("going to sleep to give csi time")
	time.Sleep(20 * time.Second)
	v, err := p.Client.GetVolume("", "", "", targetLUN.Name, false)
	if err != nil {
		return nil, err
	}
	currentMappedSdcs := []string{}
	if len(v) != 1 {
		return nil, fmt.Errorf("found %d volumes while expecting one. Target volume ID %s", len(v), targetLUN.ProviderID)
	}

	klog.Infof("current mapping %+v", v[0].MappedSdcInfo)
	if len(v[0].MappedSdcInfo) == 0 {
		klog.Errorf("found 0 Mapped SDC Info for target volume %+v", targetLUN)
		return []string{}, fmt.Errorf("found 0 Mapped SDC Info for target volume %+v", targetLUN)
	}
	for _, sdcInfo := range v[0].MappedSdcInfo {
		currentMappedSdcs = append(currentMappedSdcs, sdcInfo.SdcID)
	}
	return currentMappedSdcs, nil
}

// EnsureClonnerIgroup implements populator.StorageApi.
func (p *PowerflexClonner) EnsureClonnerIgroup(initiatorGroup string, clonnerIqn []string) (populator.MappingContext, error) {
	klog.Infof("ensuring initiator group %s for clonners %v", initiatorGroup, clonnerIqn)

	mappingContext := make(map[string]any)
	system, err := p.Client.FindSystem(p.systemId, "", "")
	if err != nil {
		return nil, err
	}
	sdcs, err := system.GetSdc()
	if err != nil {
		return nil, err
	}

	for _, sdc := range sdcs {
		if sdc.OSType != "Esx" {
			continue
		}
		klog.Infof("Comparing with sdc %+v", sdc)
		if slices.Contains(clonnerIqn, sdc.SdcGUID) {
			klog.Infof("found compatible SDC: %+v", sdc)
			mappingContext[sdcIDContextKey] = sdc.ID
			return mappingContext, nil
		}
	}
	return mappingContext, fmt.Errorf("could not find the SDC adapter on ESXI")
}

func (p *PowerflexClonner) Map(initiatorGroup string, targetLUN populator.LUN, mappingContext populator.MappingContext) (populator.LUN, error) {
	klog.Infof("mapping volume %s to initiator group %s with context %v", targetLUN.Name, initiatorGroup, mappingContext)
	sdc, volume, err := p.fetchSdcVolume(initiatorGroup, targetLUN, mappingContext)
	if err != nil {
		return targetLUN, err
	}

	mapParams := siotypes.MapVolumeSdcParam{
		SdcID:                 sdc.Sdc.ID,
		AllowMultipleMappings: "true",
	}
	err = volume.MapVolumeSdc(&mapParams)
	if err != nil {
		return targetLUN, fmt.Errorf("failed to map the volume id %s to sdc id %s: %w", volume.Volume.ID, sdc.Sdc.ID, err)
	}
	// the serial or the NAA is the {$systemID$volumeID}
	targetLUN.NAA = fmt.Sprintf("eui.%s%s", sdc.Sdc.SystemID, volume.Volume.ID)
	return targetLUN, nil
}

// Map implements populator.StorageApi.
func (p *PowerflexClonner) fetchSdcVolume(initatorGroup string, targetLUN populator.LUN, mappingContext populator.MappingContext) (*goscaleio.Sdc, *goscaleio.Volume, error) {

	// TODO rgolan do we need an instanceID as part of the client?
	// probably yes for multiple instances
	system, err := p.Client.FindSystem(p.systemId, "", "")
	if err != nil {
		return nil, nil, err
	}

	sdc, err := system.FindSdc("ID", initatorGroup)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to locate sdc by sdc guid %s", initatorGroup)
	}
	klog.Infof("found sdc name %s id %s", sdc.Sdc.Name, sdc.Sdc.ID)

	v, err := p.Client.GetVolume("", "", "", targetLUN.Name, false)
	if err != nil {
		return nil, nil, err
	}
	if len(v) != 1 {
		return nil, nil, fmt.Errorf("expected a single volume but found %d", len(v))
	}
	volumeService := goscaleio.NewVolume(p.Client)
	volumeService.Volume = v[0]
	return sdc, volumeService, nil
}

// ResolveVolumeHandleToLUN implements populator.StorageApi.
func (p *PowerflexClonner) ResolvePVToLUN(pv populator.PersistentVolume) (populator.LUN, error) {
	name := pv.VolumeAttributes["Name"]
	if name == "" {
		return populator.LUN{},
			fmt.Errorf("the PersistentVolume attribute 'Name' is empty and " +
				"essential to locate the underlying volume in PowerFlex")
	}
	id, err := p.Client.FindVolumeID(name)
	if err != nil {
		return populator.LUN{}, err

	}
	v, err := p.Client.GetVolume("", id, "", "", false)
	if err != nil {
		return populator.LUN{}, nil
	}

	if len(v) != 1 {
		return populator.LUN{}, fmt.Errorf("failed to locate a single volume by name %s", name)
	}

	klog.Infof("found volume %s", v[0].Name)
	return populator.LUN{
		Name:         v[0].Name,
		ProviderID:   v[0].ID,
		VolumeHandle: pv.VolumeHandle,
	}, nil
}

func (p *PowerflexClonner) SciniRequired() bool {
	return true
}

// UnMap implements populator.StorageApi.
func (p *PowerflexClonner) UnMap(initatorGroup string, targetLUN populator.LUN, mappingContext populator.MappingContext) error {
	unmappingAll, _ := mappingContext["UnmapAllSdc"].(bool)

	if unmappingAll {
		klog.Infof("unmapping all from volume %s", targetLUN.Name)
	} else {
		klog.Infof("unmapping volume %s from initiator group %s", targetLUN.Name, initatorGroup)
	}

	sdc, volume, err := p.fetchSdcVolume(initatorGroup, targetLUN, mappingContext)
	if err != nil {
		return err
	}

	unmapParams := siotypes.UnmapVolumeSdcParam{
		AllSdcs: "true",
	}

	if !unmappingAll {
		unmapParams = siotypes.UnmapVolumeSdcParam{
			SdcID: sdc.Sdc.ID,
		}
	}

	err = volume.UnmapVolumeSdc(&unmapParams)
	if err != nil {
		return err
	}
	return nil
}

func NewPowerflexClonner(hostname, username, password string, sslSkipVerify bool, systemId string) (PowerflexClonner, error) {
	if systemId == "" {
		return PowerflexClonner{}, fmt.Errorf("systemId is empty. Make sure to pass systemId using the env variable %q. The value can be taken from the vxflexos-config secret under the powerflex CSI deployment", SYSTEM_ID_ENV_KEY)
	}
	client, err := goscaleio.NewClientWithArgs(hostname, "", 10000, sslSkipVerify, true)
	if err != nil {
		return PowerflexClonner{}, err
	}

	_, err = client.Authenticate(&goscaleio.ConfigConnect{
		Endpoint: hostname,
		Username: username,
		Password: password,
		Insecure: sslSkipVerify,
	})
	if err != nil {
		return PowerflexClonner{}, fmt.Errorf("error authenticating: %w", err)
	}

	klog.Infof("successfuly logged in to ScaleIO Gateway at %s version %s", client.GetConfigConnect().Endpoint, client.GetConfigConnect().Version)

	return PowerflexClonner{Client: client, systemId: systemId}, nil
}
