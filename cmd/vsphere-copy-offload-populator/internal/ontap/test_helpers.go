package ontap

import (
	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/logger"
)

func NewNetappClonnerForTest(managementLIF, svm string) *NetappClonner {
	return &NetappClonner{
		managementLIF: managementLIF,
		svm:           svm,
		sslSkipVerify: true,
		log:           logger.New("ontap-test"),
	}
}
