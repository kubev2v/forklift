package infinibox

import (
	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/logger"
)

func NewInfiniboxClonnerForTest(hostname string) *InfiniboxClonner {
	return &InfiniboxClonner{
		hostname: hostname,
		insecure: true,
		log:      logger.New("infinibox-test"),
	}
}
