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
	storageContainersV4Path = "/api/clustermgmt/v4.1/config/storage-containers"
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

func (r *Client) ensurePrismConfig() error {
	if r.prismResolved {
		return nil
	}

	config, err := r.resolvePrismConfig()
	if err != nil {
		return err
	}

	r.prism = config
	r.prismResolved = true
	r.log.Info(
		"Prism endpoint resolved",
		"mode", config.Mode,
		"explicit", config.Explicit,
		"clusterUuid", config.ClusterUUID)

	return nil
}

func (r *Client) resolvePrismConfig() (PrismConfig, error) {
	clusterUUID := ""
	if r.settings != nil {
		clusterUUID = r.settings[api.NutanixClusterUUID]
	}

	if r.settings != nil {
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
		r.log.Error(err, "Prism mode detection failed, defaulting to central")
		detected = PrismCentral
	}

	return PrismConfig{
		Mode:        detected,
		ClusterUUID: clusterUUID,
		Explicit:    false,
	}, nil
}

func (r *Client) detectPrismMode() (PrismMode, error) {
	pcURL := fmt.Sprintf("%s/api/nutanix/v3/prism_central", r.url)
	var pcBody map[string]interface{}
	status, err := r.get(pcURL, &pcBody)
	if err == nil && status == http.StatusOK {
		return PrismCentral, nil
	}

	peURL := fmt.Sprintf("%s%s", r.url, storageContainersV2Path)
	var peBody map[string]interface{}
	status, err = r.get(peURL, &peBody)
	if err == nil && status == http.StatusOK {
		return PrismElement, nil
	}

	return "", liberr.New("unable to detect Prism endpoint type")
}
