package pure

import (
	"net/http"

	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/logger"
)

func NewFlashArrayClonnerForTest(hostname string, httpClient *http.Client) *FlashArrayClonner {
	return &FlashArrayClonner{
		restClient: &RestClient{
			hostname:   hostname,
			httpClient: httpClient,
			authToken:  "test-token",
			apiV2:      "2.0",
		},
		log: logger.New("pure-test"),
	}
}
