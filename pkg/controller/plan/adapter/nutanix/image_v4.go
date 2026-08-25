package nutanix

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/nutanix"
	libclient "github.com/kubev2v/forklift/pkg/lib/client/nutanix"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	libweb "github.com/kubev2v/forklift/pkg/lib/inventory/web"
)

// imagesV4Path is Prism Central's Image Service (vmm) endpoint. Prism
// Central doesn't support disk-based image creation through the v3
// "image" kind used on Prism Element (see PreTransferActions); this v4
// endpoint is its replacement.
const imagesV4Path = "/api/vmm/v4.0/content/images"

// imageV4StallTimeout is the maximum time we wait for a v4 catalog image
// to report a non-zero sizeBytes before declaring creation stalled. The v4
// Image entity has no explicit error state, so this is our only signal that
// creation failed (e.g. invalid disk reference, quota exceeded).
const imageV4StallTimeout = 30 * time.Minute

// findImageV4ByName returns the v4 image entity with the given name.
// entity is nil if no such image exists yet. The v4 Image entity has no
// explicit lifecycle state field (unlike v3's status.state), so ready
// instead reports whether sizeBytes has been populated -- Nutanix's
// signal that the image's content has finished uploading.
func (r *Client) findImageV4ByName(name string) (entity *libclient.ImageV4, ready bool, err error) {
	requestURL := fmt.Sprintf("%s%s", r.URL, imagesV4Path)
	var result libclient.V4ListResponse[libclient.ImageV4]
	status, err := r.Get(requestURL, &result, libweb.Param{Key: "$filter", Value: fmt.Sprintf("name eq '%s'", name)})
	if err != nil {
		return nil, false, liberr.Wrap(err, "image", name)
	}
	if status != http.StatusOK {
		return nil, false, liberr.New("unexpected status listing images", "image", name, "status", status)
	}
	if len(result.Data) == 0 {
		return nil, false, nil
	}
	for _, candidate := range result.Data {
		if candidate.Name == name {
			return &candidate, candidate.SizeBytes > 0, nil
		}
	}
	return nil, false, nil
}

// createImageV4 submits a v4 image creation request for a DISK_IMAGE
// sourced from the given VM disk. Creation is asynchronous; callers poll
// via findImageV4ByName.
func (r *Client) createImageV4(name, diskUUID string) error {
	body := libclient.ImageV4CreateBody(name, diskUUID)
	requestURL := fmt.Sprintf("%s%s", r.URL, imagesV4Path)
	status, err := r.Post(requestURL, body, nil)
	if err != nil {
		return liberr.Wrap(err, "disk", diskUUID, "image", name)
	}
	if status != http.StatusOK && status != http.StatusAccepted {
		return liberr.New("unexpected status creating image", "disk", diskUUID, "image", name, "status", status)
	}
	return nil
}

// deleteImageV4 deletes the v4 catalog image for a VM disk, if one
// exists. Errors are logged rather than returned; see Finalize.
func (r *Client) deleteImageV4(vmRef ref.Ref, diskUUID string) {
	name := r.migrationImageName(vmRef, diskUUID)
	entity, _, err := r.findImageV4ByName(name)
	if err != nil {
		r.Context.Log.Error(err, "Failed to look up image for cleanup", "vm", vmRef.String(), "image", name)
		return
	}
	if entity == nil {
		return
	}
	requestURL := fmt.Sprintf("%s%s/%s", r.URL, imagesV4Path, url.PathEscape(entity.ExtID))
	status, err := r.Delete(requestURL, nil)
	if err != nil || (status != http.StatusOK && status != http.StatusAccepted) {
		if err == nil {
			err = liberr.New(
				"unexpected status deleting image",
				"vm", vmRef.String(),
				"image", entity.ExtID,
				"status", status,
			)
		}
		r.Context.Log.Error(err, "Failed to delete temporary image", "vm", vmRef.String(), "image", entity.ExtID, "status", status)
	}
}

// ensureImageV4 returns whether disk's v4 catalog image has finished
// uploading, creating it first if it doesn't exist yet. Mirrors
// ensureImage's create-if-missing/poll-by-name pattern, adapted to the v4
// Image entity's lack of an explicit error state. A stalled image (sizeBytes
// still zero after imageV4StallTimeout) is treated as a creation failure.
func (r *Client) ensureImageV4(vmRef ref.Ref, disk model.Disk) (ready bool, err error) {
	name := r.migrationImageName(vmRef, disk.UUID)
	entity, ready, err := r.findImageV4ByName(name)
	if err != nil {
		return false, err
	}
	if entity == nil {
		return false, r.createImageV4(name, disk.UUID)
	}
	if ready {
		return true, nil
	}
	if entity.CreateTime != "" {
		created, parseErr := time.Parse(time.RFC3339, entity.CreateTime)
		if parseErr == nil && time.Since(created) > imageV4StallTimeout {
			return false, liberr.New(
				"Nutanix v4 image stalled: sizeBytes still zero after timeout",
				"vm", vmRef.String(),
				"disk", disk.UUID,
				"image", name,
				"age", time.Since(created).Round(time.Second).String(),
			)
		}
	}
	return false, nil
}

// resolveImageV4DownloadURL performs Prism Central's redirect handshake
// for a v4 catalog image's file download: GET .../file responds with a
// 302 pointing at the actual download location, plus an X-Redirect-Token
// header that must be replayed as a Cookie on the follow-up request. A
// generic HTTP client -- like CDI's importer -- has no way to know to do
// that on its own, so this resolves it once, up front. The cookie is kept
// in a SecretExtraHeaders Secret (see Builder.centralHTTPSource) rather
// than baked into the DataVolume spec.
func (r *Client) resolveImageV4DownloadURL(extID string) (downloadURL, cookie string, err error) {
	requestURL := fmt.Sprintf("%s%s/%s/file", r.URL, imagesV4Path, url.PathEscape(extID))
	status, header, err := r.GetNoRedirect(requestURL)
	if err != nil {
		return "", "", err
	}
	if status != http.StatusFound {
		return "", "", liberr.New("unexpected status resolving image download", "image", extID, "status", status)
	}

	location := header.Get("Location")
	token := header.Get("X-Redirect-Token")
	if location == "" || token == "" {
		return "", "", liberr.New("missing redirect location or token resolving image download", "image", extID)
	}
	return location, token, nil
}

// preferClusterExternalURL rewrites a PE entity_download Location that
// points at a CVM address so it uses the cluster's external IP (VIP)
// instead. Prism Element certificates commonly list only the VIP in
// their SAN, while PC's redirect uses the CVM IP -- which would fail
// CDI's TLS verification. The VIP serves the same path with the same
// redirect cookie.
func (r *Client) preferClusterExternalURL(downloadURL string) string {
	parsed, err := url.Parse(downloadURL)
	if err != nil || parsed.Host == "" {
		return downloadURL
	}
	vip, err := r.clusterExternalIP()
	if err != nil || vip == "" {
		return downloadURL
	}
	host, port, err := net.SplitHostPort(parsed.Host)
	if err != nil {
		host = parsed.Host
		port = ""
	}
	if host == vip {
		return downloadURL
	}
	if port != "" {
		parsed.Host = net.JoinHostPort(vip, port)
	} else {
		parsed.Host = vip
	}
	return parsed.String()
}

// clusterExternalIP returns the Prism Element cluster VIP
// (status.resources.network.external_ip). PE certificates typically
// list only that VIP in their SAN, while PC redirects downloads to a
// CVM address -- so callers rewrite the Location host to this VIP.
// Prism Central's own pseudo-cluster has no external_ip and is skipped.
func (r *Client) clusterExternalIP() (string, error) {
	result, err := libclient.ListV3[libclient.Cluster](&r.Client, "cluster", 0, 20, "")
	if err != nil {
		return "", err
	}
	var vips []string
	for _, entity := range result.Entities {
		if ip := entity.Status.Resources.Network.ExternalIP; ip != "" {
			vips = append(vips, ip)
		}
	}
	switch len(vips) {
	case 0:
		return "", nil
	case 1:
		return vips[0], nil
	default:
		// Multiple managed clusters: rewriting to an arbitrary VIP risks
		// sending the download to the wrong cluster.
		return "", nil
	}
}
