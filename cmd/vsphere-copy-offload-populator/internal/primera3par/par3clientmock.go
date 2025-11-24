package primera3par

import (
	"context"
	"fmt"
	"log"

	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/populator"
)

type MockPrimera3ParClient struct {
	SessionKey string
	Volumes    map[string]populator.LUN
	VLUNs      map[string][]VLun
	Hosts      map[string]string
	HostSets   map[string][]string
}

func NewMockPrimera3ParClient() *MockPrimera3ParClient {
	return &MockPrimera3ParClient{
		SessionKey: "mock-session-key",
		Volumes:    make(map[string]populator.LUN),
		VLUNs:      make(map[string][]VLun),
		Hosts:      make(map[string]string),
		HostSets:   make(map[string][]string),
	}
}

func (m *MockPrimera3ParClient) GetSessionKey() (string, error) {
	log.Println("Mock: GetSessionKey called")
	return m.SessionKey, nil
}

func (m *MockPrimera3ParClient) EnsureHostsWithIds(iqn []string) ([]string, error) {
	for hostName, existingIQN := range m.Hosts {
		if existingIQN == iqn[0] {
			return []string{hostName}, nil
		}
	}

	hostName := fmt.Sprintf("mock-host-%s", iqn)
	m.Hosts[hostName] = iqn[0]
	log.Printf("Mock: Created host %s with IQN %s", hostName, iqn)
	return []string{hostName}, nil
}

func (m *MockPrimera3ParClient) EnsureHostSetExists(hostSetName string) error {
	if _, exists := m.HostSets[hostSetName]; !exists {
		m.HostSets[hostSetName] = []string{}
		log.Printf("Mock: Created host set %s", hostSetName)
	}
	return nil
}

func (m *MockPrimera3ParClient) AddHostToHostSet(hostSetName string, hostName string) error {
	if _, exists := m.HostSets[hostSetName]; !exists {
		return fmt.Errorf("mock: host set %s does not exist", hostSetName)
	}

	for _, existingHost := range m.HostSets[hostSetName] {
		if existingHost == hostName {
			return nil
		}
	}

	m.HostSets[hostSetName] = append(m.HostSets[hostSetName], hostName)
	log.Printf("Mock: Added host %s to host set %s", hostName, hostSetName)
	return nil
}

func (m *MockPrimera3ParClient) EnsureLunMapped(initiatorGroup string, targetLUN populator.LUN) (populator.LUN, error) {
	if _, exists := m.Volumes[targetLUN.Name]; !exists {
		return populator.LUN{}, fmt.Errorf("mock: volume %s does not exist", targetLUN.Name)
	}

	vlun := VLun{
		VolumeName: targetLUN.Name,
		LUN:        len(m.VLUNs[initiatorGroup]) + 1,
		Hostname:   initiatorGroup,
	}

	m.VLUNs[initiatorGroup] = append(m.VLUNs[initiatorGroup], vlun)
	log.Printf("Mock: EnsureLunMapped -> Volume %s mapped to initiator group %s with LUN ID %d", targetLUN.Name, initiatorGroup, vlun.LUN)
	return targetLUN, nil
}

func (m *MockPrimera3ParClient) LunUnmap(ctx context.Context, initiatorGroupName string, lunName string) error {
	vluns, exists := m.VLUNs[initiatorGroupName]
	if !exists {
		return fmt.Errorf("mock: no VLUNs found for initiator group %s", initiatorGroupName)
	}

	for i, vlun := range vluns {
		if vlun.VolumeName == lunName {
			m.VLUNs[initiatorGroupName] = append(vluns[:i], vluns[i+1:]...)
			log.Printf("Mock: LunUnmap -> Volume %s unmapped from initiator group %s", lunName, initiatorGroupName)
			return nil
		}
	}

	return fmt.Errorf("mock: LUN %s not found for initiator group %s", lunName, initiatorGroupName)
}

func (m *MockPrimera3ParClient) GetLunDetailsByVolumeName(lunName string, lun populator.LUN) (populator.LUN, error) {
	if volume, exists := m.Volumes[lunName]; exists {
		log.Printf("Mock: GetLunDetailsByVolumeName -> Found volume %s", lunName)
		return volume, nil
	}

	return populator.LUN{}, fmt.Errorf("mock: volume %s not found", lunName)
}

func (m *MockPrimera3ParClient) CurrentMappedGroups(volumeName string, mappingContext populator.MappingContext) ([]string, error) {
	var groups []string

	for group, vluns := range m.VLUNs {
		for _, vlun := range vluns {
			if vlun.VolumeName == volumeName {
				groups = append(groups, group)
			}
		}
	}

	if len(groups) == 0 {
		return nil, fmt.Errorf("mock: no mapped groups found for volume %s", volumeName)
	}

	log.Printf("Mock: CurrentMappedGroups -> Volume %s is mapped to groups: %v", volumeName, groups)
	return groups, nil
}
