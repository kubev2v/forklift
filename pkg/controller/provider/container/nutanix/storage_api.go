package nutanix

import (
	"fmt"
	"net/http"

	liberr "github.com/kubev2v/forklift/pkg/lib/error"
)

func (r *Client) listStorageContainers() (entities []map[string]interface{}, err error) {
	if err = r.ensurePrismConfig(); err != nil {
		return nil, err
	}

	switch r.prism.Mode {
	case PrismElement:
		return r.listStorageContainersElement()
	case PrismCentral:
		return r.listStorageContainersCentral()
	default:
		return nil, liberr.New("unknown Prism mode", "mode", r.prism.Mode)
	}
}

func (r *Client) listStorageContainersElement() ([]map[string]interface{}, error) {
	url := fmt.Sprintf("%s%s", r.url, storageContainersV2Path)
	result := make(map[string]interface{})
	status, err := r.get(url, &result)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, liberr.New(fmt.Sprintf("unexpected status listing storage containers: %d", status))
	}

	rawEntities, err := extractMapList(result, "entities")
	if err != nil {
		return nil, err
	}

	entities := make([]map[string]interface{}, 0, len(rawEntities))
	for _, raw := range rawEntities {
		entities = append(entities, storageContainerEntityFromV2(raw))
	}

	return filterStorageContainersByCluster(entities, r.prism.ClusterUUID), nil
}

func (r *Client) listStorageContainersCentral() ([]map[string]interface{}, error) {
	url := fmt.Sprintf("%s%s", r.url, storageContainersV4Path)
	result := make(map[string]interface{})
	status, err := r.get(url, &result)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, liberr.New(fmt.Sprintf("unexpected status listing storage containers: %d", status))
	}

	rawEntities, err := extractMapList(result, "data")
	if err != nil {
		return nil, err
	}

	entities := make([]map[string]interface{}, 0, len(rawEntities))
	for _, raw := range rawEntities {
		entities = append(entities, storageContainerEntityFromV4(raw))
	}

	return filterStorageContainersByCluster(entities, r.prism.ClusterUUID), nil
}
