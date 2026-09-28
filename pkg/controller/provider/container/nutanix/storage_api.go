package nutanix

import (
	"fmt"
	"net/http"

	liberr "github.com/kubev2v/forklift/pkg/lib/error"
)

func (r *Client) listStorageContainers() (entities []storageContainerEntity, err error) {
	switch r.prism.Mode {
	case PrismElement:
		return r.listStorageContainersElement()
	case PrismCentral:
		return r.listStorageContainersCentral()
	default:
		return nil, liberr.New("unknown Prism mode", "mode", r.prism.Mode)
	}
}

func (r *Client) listStorageContainersElement() ([]storageContainerEntity, error) {
	url := fmt.Sprintf("%s%s", r.url, storageContainersV2Path)
	var result struct {
		Entities []storageContainerV2Raw `json:"entities"`
	}
	status, err := r.get(url, &result)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, liberr.New(fmt.Sprintf("unexpected status listing storage containers: %d", status))
	}

	entities := make([]storageContainerEntity, 0, len(result.Entities))
	for _, raw := range result.Entities {
		entities = append(entities, raw.toEntity())
	}

	return filterStorageContainersByCluster(entities, r.prism.ClusterUUID), nil
}

func (r *Client) listStorageContainersCentral() ([]storageContainerEntity, error) {
	rawEntities, err := listAllV4[storageContainerV4Raw](r, storageContainersV4Path, storageContainerV4PageSize)
	if err != nil {
		return nil, err
	}

	entities := make([]storageContainerEntity, 0, len(rawEntities))
	for _, raw := range rawEntities {
		entities = append(entities, raw.toEntity())
	}

	return filterStorageContainersByCluster(entities, r.prism.ClusterUUID), nil
}
