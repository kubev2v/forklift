package nutanix

import (
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
)

// listImages lists VM images/ISOs, dispatching to the Prism mode-specific
// implementation. Images are not scoped by clusterUuid in either mode: an
// image can be shared across every cluster it's registered to (there is no
// single owning cluster_reference), which is also why model.Image has no
// Cluster field.
func (r *Client) listImages() (entities []imageEntity, err error) {
	switch r.prism.Mode {
	case PrismElement:
		return r.listImagesElement()
	case PrismCentral:
		return r.listImagesCentral()
	default:
		return nil, liberr.New("unknown Prism mode", "mode", r.prism.Mode)
	}
}

// listImagesElement lists images via the v3 "image" kind, which Prism
// Element serves directly.
func (r *Client) listImagesElement() ([]imageEntity, error) {
	return listAllV3[imageEntity](r, "image", "", imagePageSize)
}

// listImagesCentral lists images via Prism Central's Image Service (vmm v4
// content/images collection). The v3 "image" kind is also reachable through
// Prism Central, but it isn't reliably populated there -- images registered
// with PC's image service only show up via this v4 endpoint.
func (r *Client) listImagesCentral() ([]imageEntity, error) {
	rawEntities, err := listAllV4[imageV4Raw](r, imagesV4Path, imageV4PageSize)
	if err != nil {
		return nil, err
	}

	images := make([]imageEntity, 0, len(rawEntities))
	for _, raw := range rawEntities {
		images = append(images, raw.toEntity())
	}

	return images, nil
}
