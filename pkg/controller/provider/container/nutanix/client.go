package nutanix

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/kubev2v/forklift/pkg/controller/base"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	libweb "github.com/kubev2v/forklift/pkg/lib/inventory/web"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	"github.com/kubev2v/forklift/pkg/lib/util"
	core "k8s.io/api/core/v1"
)

// Settings
const (
	// Connect retry delay.
	RetryDelay = time.Second * 5
	// Connection timeout.
	ConnectionTimeout = 30 * time.Second
	// Default API port
	DefaultPort = "9440"
)

// Per-request page sizes for v3 list endpoints. listAll() pages through as
// many requests as needed regardless of these values; they only bound how
// many entities are requested per page.
const (
	clusterPageSize = 100
	hostPageSize    = 1000
	vmPageSize      = 100
	subnetPageSize  = 500
	imagePageSize   = 500
)

// Not found error.
type NotFound struct {
}

func (e *NotFound) Error() string {
	return "not found."
}

// Nutanix API Client
type Client struct {
	// Base URL (e.g., https://prism-central:9440)
	url string
	// HTTP client
	client *libweb.Client
	// Secret containing credentials
	secret *core.Secret
	// Provider settings (prismType, clusterUuid, ...)
	settings map[string]string
	// Client timeout
	clientTimeout time.Duration
	// Logger
	log logging.LevelLogger
	// Resolved Prism endpoint configuration.
	prism PrismConfig
	// Whether prism config has been resolved.
	prismResolved bool
}

// Connect and authenticate with Nutanix Prism
func (r *Client) connect() (status int, err error) {
	var TLSClientConfig *tls.Config

	if r.client != nil {
		return http.StatusOK, nil
	}

	// Configure TLS
	if base.GetInsecureSkipVerifyFlag(r.secret) {
		TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	} else if cacert, found := util.GetCACert(r.secret); found {
		roots := x509.NewCertPool()
		ok := roots.AppendCertsFromPEM(cacert)
		if !ok {
			err = liberr.New("failed to parse CA certificate")
			return http.StatusBadRequest, err
		}
		TLSClientConfig = &tls.Config{RootCAs: roots}
	} else {
		TLSClientConfig = &tls.Config{InsecureSkipVerify: false}
	}

	r.url = strings.TrimRight(r.url, "/")

	// Create HTTP client
	r.client = &libweb.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 10 * time.Second,
			}).DialContext,
			MaxIdleConns:          10,
			IdleConnTimeout:       10 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			TLSClientConfig:       TLSClientConfig,
		},
	}

	// Test connection by listing clusters
	status, err = r.testConnection()
	if err != nil {
		r.client = nil
		return status, err
	}

	if err = r.ensurePrismConfig(); err != nil {
		r.client = nil
		return status, err
	}

	r.log.Info("Successfully connected to Nutanix",
		"url", r.url,
		"prismMode", r.prism.Mode)

	return http.StatusOK, nil
}

// Test connection to Nutanix API
func (r *Client) testConnection() (status int, err error) {
	// Test by listing clusters (minimal API call)
	url := fmt.Sprintf("%s/api/nutanix/v3/clusters/list", r.url)

	// Create a simple list request
	body := map[string]interface{}{
		"kind":   "cluster",
		"offset": 0,
		"length": 1,
	}

	status, err = r.post(url, body, nil)
	if err != nil {
		return status, liberr.Wrap(err, "connection test failed")
	}

	return http.StatusOK, nil
}

// GET request
func (r *Client) get(url string, object interface{}, params ...libweb.Param) (status int, err error) {
	status, err = r.connect()
	if err != nil {
		return
	}

	// Set Basic Auth header
	r.client.Header = r.createAuthHeader()

	status, err = r.client.Get(url, object, params...)
	if err != nil {
		return
	}

	return
}

// POST request (Nutanix uses POST for list operations)
func (r *Client) post(url string, body interface{}, object interface{}) (status int, err error) {
	status, err = r.connect()
	if err != nil {
		return
	}

	// Set Basic Auth header
	r.client.Header = r.createAuthHeader()

	// Use the client's Post method
	status, err = r.client.Post(url, body, object)
	if err != nil {
		return status, err
	}

	return
}

// listAll pages through a v3 list endpoint, following the response's
// total_matches, and returns every entity across all pages. This keeps a
// single provider from silently truncating on Prism inventories larger than
// one page.
func (r *Client) listAll(resourceKind string, filter map[string]interface{}, pageSize int) (entities []map[string]interface{}, err error) {
	offset := 0
	entities = make([]map[string]interface{}, 0)

	for {
		result, err := r.list(resourceKind, filter, offset, pageSize)
		if err != nil {
			return nil, err
		}

		entitiesList, ok := result["entities"].([]interface{})
		if !ok {
			break
		}
		for _, e := range entitiesList {
			if entity, ok := e.(map[string]interface{}); ok {
				entities = append(entities, entity)
			}
		}

		// No entities came back; nothing left to page through.
		if len(entitiesList) == 0 {
			break
		}

		metadata, ok := result["metadata"].(map[string]interface{})
		if !ok {
			break
		}
		totalMatches, ok := metadata["total_matches"].(float64)
		if !ok {
			break
		}

		offset += len(entitiesList)
		if offset >= int(totalMatches) {
			break
		}
	}

	return entities, nil
}

// List resources using Nutanix v3 API pattern
// Nutanix uses POST for list operations with a filter body
func (r *Client) list(resourceKind string, filter map[string]interface{}, offset, length int) (result map[string]interface{}, err error) {
	url := fmt.Sprintf("%s/api/nutanix/v3/%ss/list", r.url, resourceKind)

	body := map[string]interface{}{
		"kind":   resourceKind,
		"offset": offset,
		"length": length,
	}

	// Add filter if provided
	if filter != nil {
		body["filter"] = filter
	}

	result = make(map[string]interface{})
	status, err := r.post(url, body, &result)
	if err != nil {
		return nil, err
	}

	if status != http.StatusOK {
		return nil, liberr.New(fmt.Sprintf("unexpected status: %d", status))
	}

	return result, nil
}

// Get resource by UUID
func (r *Client) getResource(resourceKind, uuid string) (result map[string]interface{}, err error) {
	url := fmt.Sprintf("%s/api/nutanix/v3/%ss/%s", r.url, resourceKind, uuid)

	result = make(map[string]interface{})
	status, err := r.get(url, &result)
	if err != nil {
		return nil, err
	}

	if status == http.StatusNotFound {
		return nil, &NotFound{}
	}

	if status != http.StatusOK {
		return nil, liberr.New(fmt.Sprintf("unexpected status: %d", status))
	}

	return result, nil
}

// Create HTTP Header with Basic Auth
func (r *Client) createAuthHeader() http.Header {
	user := string(r.secret.Data["user"])
	password := string(r.secret.Data["password"])

	header := http.Header{}
	header.Set("Content-Type", "application/json")
	header.Set("Authorization", "Basic "+basicAuth(user, password))

	return header
}

// Encode Basic Auth credentials
func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

// List all clusters, scoped to the configured clusterUuid (if any).
func (r *Client) listClusters() (entities []map[string]interface{}, err error) {
	entities, err = r.listAll("cluster", nil, clusterPageSize)
	if err != nil {
		return nil, err
	}
	if err = r.ensurePrismConfig(); err != nil {
		return nil, err
	}
	return filterEntitiesByCluster(entities, r.prism.ClusterUUID, "metadata.uuid"), nil
}

// List all hosts, scoped to the configured clusterUuid (if any).
func (r *Client) listHosts() (entities []map[string]interface{}, err error) {
	entities, err = r.listAll("host", nil, hostPageSize)
	if err != nil {
		return nil, err
	}
	if err = r.ensurePrismConfig(); err != nil {
		return nil, err
	}
	return filterEntitiesByCluster(entities, r.prism.ClusterUUID, "status.resources.cluster_reference.uuid"), nil
}

// List all VMs, scoped to the configured clusterUuid (if any).
func (r *Client) listVMs() (entities []map[string]interface{}, err error) {
	entities, err = r.listAll("vm", nil, vmPageSize)
	if err != nil {
		return nil, err
	}
	if err = r.ensurePrismConfig(); err != nil {
		return nil, err
	}
	return filterEntitiesByCluster(entities, r.prism.ClusterUUID, "spec.cluster_reference.uuid"), nil
}

// List all subnets (networks), scoped to the configured clusterUuid (if any).
func (r *Client) listSubnets() (entities []map[string]interface{}, err error) {
	entities, err = r.listAll("subnet", nil, subnetPageSize)
	if err != nil {
		return nil, err
	}
	if err = r.ensurePrismConfig(); err != nil {
		return nil, err
	}
	return filterEntitiesByCluster(entities, r.prism.ClusterUUID, "status.resources.cluster_reference.uuid"), nil
}

// List all images. Images are not scoped by clusterUuid: on Prism Central an
// image can be shared across every cluster it's registered to (there is no
// single owning cluster_reference), which is also why model.Image has no
// Cluster field.
func (r *Client) listImages() (entities []map[string]interface{}, err error) {
	return r.listAll("image", nil, imagePageSize)
}
