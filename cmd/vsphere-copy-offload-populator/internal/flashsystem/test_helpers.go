package flashsystem

import (
	"net/http"

	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/logger"
)

func NewFlashSystemClonnerForTest(managementIP, port string, httpClient *http.Client) *FlashSystemClonner {
	return &FlashSystemClonner{
		api: &FlashSystemAPIClient{
			ManagementIP: managementIP,
			Port:         port,
			httpClient:   httpClient,
			authToken:    "test-token",
			log:          logger.New("flashsystem-test"),
		},
		log: logger.New("flashsystem-test"),
	}
}
