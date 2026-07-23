package powerflex

import (
	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/logger"
)

// NewPowerflexClonnerForTest creates a minimal PowerflexClonner with only the
// systemId set, for use in unit tests that validate MatchesDevice.
func NewPowerflexClonnerForTest(systemId string) *PowerflexClonner {
	return &PowerflexClonner{
		systemId: systemId,
		log:      logger.New("powerflex-test"),
	}
}
