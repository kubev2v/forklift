package powerstore

import (
	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/logger"
)

func NewPowerstoreClonnerForTest(hostname string) *PowerstoreClonner {
	return &PowerstoreClonner{
		hostname:      hostname,
		sslSkipVerify: true,
		log:           logger.New("powerstore-test"),
	}
}
