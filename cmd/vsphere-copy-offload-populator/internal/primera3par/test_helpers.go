package primera3par

import (
	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/logger"
)

func NewPrimera3ParClonnerForTest(client Primera3ParClient) *Primera3ParClonner {
	return &Primera3ParClonner{
		client: client,
		log:    logger.New("primera3par-test"),
	}
}
