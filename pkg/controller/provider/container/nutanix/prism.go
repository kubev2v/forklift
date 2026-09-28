package nutanix

import (
	"fmt"
	"net/http"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
)

const (
	prismCentralPath        = "/api/nutanix/v3/prism_central"
	storageContainersV2Path = "/api/nutanix/v2.0/storage_containers"
	storageContainersV4Path = "/api/clustermgmt/v4.0/config/storage-containers"
	// imagesV4Path is Prism Central's Image Service (vmm) list endpoint.
	// The v3 "image" kind (used via listAll("image", ...) on Prism Element)
	// isn't reliably populated when queried through Prism Central; images
	// registered with PC's image service only show up here.
	imagesV4Path = "/api/vmm/v4.0/content/images"
)

// PrismMode identifies whether the provider URL targets Prism Central or Element.
type PrismMode string

const (
	PrismCentral PrismMode = api.NutanixPrismCentral
	PrismElement PrismMode = api.NutanixPrismElement
)

// PrismConfig holds the resolved Prism endpoint configuration.
type PrismConfig struct {
	Mode        PrismMode
	ClusterUUID string
	Explicit    bool
}

func parsePrismMode(value string) (PrismMode, error) {
	switch value {
	case api.NutanixPrismCentral:
		return PrismCentral, nil
	case api.NutanixPrismElement:
		return PrismElement, nil
	default:
		return "", liberr.New("invalid prismType setting", "value", value)
	}
}

func (r *Client) resolvePrismConfig() (PrismConfig, error) {
	clusterUUID := ""
	if r.settings != nil {
		clusterUUID = r.settings[api.NutanixClusterUUID]
		if raw, ok := r.settings[api.NutanixPrismType]; ok && raw != "" {
			mode, err := parsePrismMode(raw)
			if err != nil {
				return PrismConfig{}, err
			}

			return PrismConfig{
				Mode:        mode,
				ClusterUUID: clusterUUID,
				Explicit:    true,
			}, nil
		}
	}

	detected, err := r.detectPrismMode()
	if err != nil {
		return PrismConfig{}, liberr.Wrap(err, "Prism mode auto-detection failed")
	}

	return PrismConfig{
		Mode:        detected,
		ClusterUUID: clusterUUID,
		Explicit:    false,
	}, nil
}

// detectPrismMode probes the API to tell Prism Central and Prism Element
// apart. It is only reached from resolvePrismConfig() during connect(),
// after the shared client is connected, and calls the shared client
// directly rather than through this package's get() wrapper.
func (r *Client) detectPrismMode() (PrismMode, error) {
	r.ensureWebClient()

	pcURL := fmt.Sprintf("%s/api/nutanix/v3/prism_central", r.url)
	var pcBody map[string]interface{}
	pcStatus, pcErr := r.web.Get(pcURL, &pcBody)
	if pcErr == nil && pcStatus == http.StatusOK {
		return PrismCentral, nil
	}

	peURL := fmt.Sprintf("%s%s", r.url, storageContainersV2Path)
	var peBody map[string]interface{}
	peStatus, peErr := r.web.Get(peURL, &peBody)
	if peErr == nil && peStatus == http.StatusOK {
		return PrismElement, nil
	}

	return "", liberr.New(
		"unable to detect Prism endpoint type",
		"prismCentralStatus", pcStatus,
		"prismCentralError", pcErr,
		"prismElementStatus", peStatus,
		"prismElementError", peErr)
}
