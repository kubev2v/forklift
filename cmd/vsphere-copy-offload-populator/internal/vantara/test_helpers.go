package vantara

import (
	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/logger"
)

// MockVantaraClientForTest is a minimal VantaraClient implementation for
// testing MatchesDevice from outside the vantara package.
type MockVantaraClientForTest struct {
	LdevResp       *LdevResponse
	LdevStatusCode int
	LdevErr        error
	LastLdevID     string
}

func (m *MockVantaraClientForTest) Connect() error    { return nil }
func (m *MockVantaraClientForTest) Disconnect() error { return nil }

func (m *MockVantaraClientForTest) GetLdev(ldevId string) (*LdevResponse, int, error) {
	m.LastLdevID = ldevId
	return m.LdevResp, m.LdevStatusCode, m.LdevErr
}

func (m *MockVantaraClientForTest) AddPath(ldevId, portId, hostGroupNumber string) error {
	return nil
}

func (m *MockVantaraClientForTest) DeletePath(ldevId, portId, hostGroupNumber, lunId string) error {
	return nil
}

func (m *MockVantaraClientForTest) GetPortDetails() (*PortDetailsResponse, error) {
	return &PortDetailsResponse{}, nil
}

func (m *MockVantaraClientForTest) CreateCloneLdev(snapshotGroupName, snapshotPoolId, pvolLdevId, svolLdevId, copySpeed string) error {
	return nil
}

func (m *MockVantaraClientForTest) GetClonePairs(snapshotGroupName, pvolLdevId string) (*ClonePairResponse, error) {
	return &ClonePairResponse{}, nil
}

func (m *MockVantaraClientForTest) GetStorageInfo() (*StorageInfo, error) {
	return &StorageInfo{}, nil
}

func NewVantaraClonnerForTest(client VantaraClient) *VantaraCloner {
	return &VantaraCloner{
		client: client,
		log:    logger.New("vantara-test"),
	}
}
