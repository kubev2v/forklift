package powerflex

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/dell/goscaleio"
	siotypes "github.com/dell/goscaleio/types/v1"
	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/populator"
	"k8s.io/klog/v2"
)

const (
	SYSTEM_ID_ENV_KEY        = "POWERFLEX_SYSTEM_ID"
	sdcIDContextKey   string = "sdcId"
	loggerName               = "copy-offload"
)

type PowerflexClonner struct {
	Client    *goscaleio.Client
	systemId  string
	sdcId     string
	arrayInfo populator.StorageArrayInfo
}

// Ensure PowerflexClonner implements StorageArrayInfoProvider
var _ populator.StorageArrayInfoProvider = &PowerflexClonner{}

// GetStorageArrayInfo returns metadata about the PowerFlex array for metric labels.
func (p *PowerflexClonner) GetStorageArrayInfo() populator.StorageArrayInfo {
	return p.arrayInfo
}

// CurrentMappedGroups implements populator.StorageApi.
func (p *PowerflexClonner) CurrentMappedGroups(targetLUN populator.LUN, mappingContext populator.MappingContext) ([]string, error) {
	log := klog.Background().WithName(loggerName).WithName("map")
	log.V(2).Info("querying current mapped groups", "volume", targetLUN.Name)

	csiGraceSeconds := 20
	log.V(2).Info("waiting for PowerFlex CSI to refresh data", "sleep_seconds", csiGraceSeconds)
	time.Sleep(time.Second * time.Duration(csiGraceSeconds))

	v, err := p.Client.GetVolume("", "", "", targetLUN.Name, false)
	if err != nil {
		return nil, err
	}
	currentMappedSdcs := []string{}
	if len(v) != 1 {
		return nil, fmt.Errorf("found %d volumes while expecting one. Target volume ID %s", len(v), targetLUN.ProviderID)
	}

	log.V(2).Info("current mapping info", "volume", targetLUN.Name, "mapped_sdcs", v[0].MappedSdcInfo)
	if len(v[0].MappedSdcInfo) == 0 {
		// although shouldn't happen we should not break the flow here.
		log.V(2).Info("no SDC mappings found for volume", "volume", targetLUN.Name)
	}
	for _, sdcInfo := range v[0].MappedSdcInfo {
		currentMappedSdcs = append(currentMappedSdcs, sdcInfo.SdcID)
	}

	log.V(2).Info("found mapped groups", "volume", targetLUN.Name, "sdc_ids", currentMappedSdcs)
	return currentMappedSdcs, nil
}

// EnsureClonnerIgroup implements populator.StorageApi.
func (p *PowerflexClonner) EnsureClonnerIgroup(initiatorGroup string, clonnerIqn []string) (populator.MappingContext, error) {
	log := klog.Background().WithName(loggerName).WithName("map").WithName("ensure-igroup")
	log.Info("ensuring initiator group", "group", initiatorGroup, "adapters", clonnerIqn)

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
		log.V(2).Info("comparing adapter with SDC GUID", "adapters", clonnerIqn, "sdc_guid", sdc.SdcGUID)
		if slices.ContainsFunc(clonnerIqn, func(e string) bool {
			return strings.EqualFold(e, sdc.SdcGUID)
		}) {
			log.Info("found compatible SDC", "sdc_name", sdc.Name, "sdc_id", sdc.ID, "sdc_guid", sdc.SdcGUID)
			mappingContext[sdcIDContextKey] = sdc.ID
			p.sdcId = sdc.ID
			log.Info("initiator group ready", "group", initiatorGroup, "sdc_id", sdc.ID)
			return mappingContext, nil
		}
	}
	return mappingContext, fmt.Errorf("could not find the SDC adapter on ESXI")
}

func (p *PowerflexClonner) MapTarget(targetLUN populator.LUN, mappingContext populator.MappingContext) (populator.LUN, error) {
	return p.Map(p.sdcId, targetLUN, mappingContext)
}

func (p *PowerflexClonner) UnmapTarget(targetLUN populator.LUN, mappingContext populator.MappingContext) error {
	return p.UnMap(p.sdcId, targetLUN, mappingContext)
}

func (p *PowerflexClonner) Map(initiatorGroup string, targetLUN populator.LUN, mappingContext populator.MappingContext) (populator.LUN, error) {
	log := klog.Background().WithName(loggerName).WithName("map")
	log.Info("mapping volume to group", "volume", targetLUN.Name, "group", initiatorGroup)

	sdc, volume, err := p.fetchSdcVolume(initiatorGroup, targetLUN, mappingContext)
	if err != nil {
		return targetLUN, err
	}

	mapParams := siotypes.MapVolumeSdcParam{
		SdcID:                 sdc.Sdc.ID,
		AllowMultipleMappings: "true",
	}
	log.V(2).Info("mapping volume to SDC", "volume_id", volume.Volume.ID, "sdc_id", sdc.Sdc.ID)
	err = volume.MapVolumeSdc(&mapParams)
	if err != nil {
		return targetLUN, fmt.Errorf("failed to map the volume id %s to sdc id %s: %w", volume.Volume.ID, sdc.Sdc.ID, err)
	}
	// the serial or the NAA is the {$systemID$volumeID}
	targetLUN.NAA = fmt.Sprintf("eui.%s%s", sdc.Sdc.SystemID, volume.Volume.ID)
	log.Info("volume mapped successfully", "volume", targetLUN.Name, "naa", targetLUN.NAA)
	return targetLUN, nil
}

// Map implements populator.StorageApi.
func (p *PowerflexClonner) fetchSdcVolume(initatorGroup string, targetLUN populator.LUN, mappingContext populator.MappingContext) (*goscaleio.Sdc, *goscaleio.Volume, error) {
	log := klog.Background().WithName(loggerName).WithName("map")

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
	log.V(2).Info("found SDC", "sdc_name", sdc.Sdc.Name, "sdc_id", sdc.Sdc.ID)

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
	log := klog.Background().WithName(loggerName).WithName("resolve")
	log.Info("resolving PV to LUN", "pv", pv.Name, "volume_handle", pv.VolumeHandle)

	name := pv.VolumeAttributes["Name"]
	if name == "" {
		return populator.LUN{},
			fmt.Errorf("the PersistentVolume attribute 'Name' is empty and " +
				"essential to locate the underlying volume in PowerFlex")
	}

	log.V(2).Info("looking up volume by name", "name", name)
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

	lun := populator.LUN{
		Name:         v[0].Name,
		ProviderID:   v[0].ID,
		VolumeHandle: pv.VolumeHandle,
	}
	log.Info("LUN resolved", "lun", lun.Name, "provider_id", lun.ProviderID)
	return lun, nil
}

func (p *PowerflexClonner) SciniRequired() bool {
	return true
}

// UnMap implements populator.StorageApi.
func (p *PowerflexClonner) UnMap(initatorGroup string, targetLUN populator.LUN, mappingContext populator.MappingContext) error {
	log := klog.Background().WithName(loggerName).WithName("map")
	unmappingAll, _ := mappingContext["UnmapAllSdc"].(bool)

	if unmappingAll {
		log.Info("unmapping all SDCs from volume", "volume", targetLUN.Name)
	} else {
		log.Info("unmapping volume from group", "volume", targetLUN.Name, "group", initatorGroup)
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

	log.V(2).Info("unmapping volume from SDC", "volume_id", volume.Volume.ID, "sdc_id", sdc.Sdc.ID, "all_sdcs", unmappingAll)
	err = volume.UnmapVolumeSdc(&unmapParams)
	if err != nil {
		return err
	}

	log.Info("volume unmapped successfully", "volume", targetLUN.Name)
	return nil
}

func NewPowerflexClonner(hostname, username, password string, sslSkipVerify bool, systemId string) (PowerflexClonner, error) {
	log := klog.Background().WithName(loggerName).WithName("setup")

	if systemId == "" {
		return PowerflexClonner{}, fmt.Errorf("systemId is empty. Make sure to pass systemId using the env variable %q. The value can be taken from the vxflexos-config secret under the powerflex CSI deployment", SYSTEM_ID_ENV_KEY)
	}

	log.V(2).Info("creating PowerFlex client", "hostname", hostname, "system_id", systemId)
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

	log.V(2).Info("authenticated to PowerFlex gateway", "endpoint", client.GetConfigConnect().Endpoint)

	return PowerflexClonner{
		Client:   client,
		systemId: systemId,
		arrayInfo: populator.StorageArrayInfo{
			Vendor:  "Dell",
			Product: "PowerFlex",
		},
	}, nil
}
