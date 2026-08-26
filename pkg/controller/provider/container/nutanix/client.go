package nutanix

import (
	"net/http"
	"time"

	nutanixweb "github.com/kubev2v/forklift/pkg/lib/client/nutanix"
	libweb "github.com/kubev2v/forklift/pkg/lib/inventory/web"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	core "k8s.io/api/core/v1"
)

// Settings
const (
	// Connect retry delay.
	RetryDelay = time.Second * 5
	// Connection timeout.
	ConnectionTimeout = nutanixweb.ConnectionTimeout
)

// Per-request page sizes for v3 list endpoints. ListAllV3 pages through as
// many requests as needed regardless of these values; they only bound how
// many entities are requested per page.
const (
	clusterPageSize = 100
	hostPageSize    = 1000
	vmPageSize      = 100
	subnetPageSize  = 500
	imagePageSize   = 500
	// Per-request page sizes for v4 "config"/"content" namespace endpoints.
	// ListAllV4 pages through as many requests as needed regardless of
	// these values; the v4 image endpoint additionally caps $limit at 100.
	storageContainerV4PageSize = 100
	imageV4PageSize            = 100
)

// Client wraps the shared pkg/lib/client/nutanix REST client with the
// collector-specific concerns: Prism mode resolution and the
// cluster-scoped, entity-specific list methods used by the collector.
// The owner must call connect() once before any list method; collect()
// does this at the start of each inventory pass.
type Client struct {
	// Base URL (e.g., https://prism-central:9440)
	url string
	// Secret containing credentials
	secret *core.Secret
	// Provider settings (prismType, clusterUuid, ...)
	settings map[string]string
	// Client timeout
	clientTimeout time.Duration
	// Logger
	log logging.LevelLogger
	// Resolved Prism endpoint configuration, set once by connect().
	prism PrismConfig
	// Whether connect() has completed successfully.
	connected bool

	// Shared REST client (connect/auth/get/post/pagination).
	web nutanixweb.Client
}

// ensureWebClient populates the shared REST client from this client's
// fields the first time it's needed. It never resets an already-populated
// r.web, so a live connection (and its TLS-configured transport) survives
// repeated calls.
func (r *Client) ensureWebClient() {
	if r.web.URL == "" {
		r.web = nutanixweb.Client{
			URL:     r.url,
			Secret:  r.secret,
			Timeout: r.clientTimeout,
			Log:     r.log,
		}
	}
}

// Connect and authenticate with Nutanix Prism, then resolve the Prism
// mode (Central vs Element) for this provider.
func (r *Client) connect() (status int, err error) {
	if r.connected {
		return http.StatusOK, nil
	}

	r.ensureWebClient()

	status, err = r.web.Connect()
	if err != nil {
		return
	}
	// Pick up the trimmed (no trailing slash) URL.
	r.url = r.web.URL

	config, err := r.resolvePrismConfig()
	if err != nil {
		return status, err
	}
	r.prism = config
	r.log.Info(
		"Prism endpoint resolved",
		"mode", config.Mode,
		"explicit", config.Explicit,
		"clusterUuid", config.ClusterUUID)

	r.connected = true
	r.log.Info("Successfully connected to Nutanix",
		"url", r.url,
		"prismMode", r.prism.Mode)

	return status, nil
}

// GET request. Requires a prior successful connect().
func (r *Client) get(url string, object any, params ...libweb.Param) (status int, err error) {
	return r.web.Get(url, object, params...)
}

// listAllV3 pages through a v3 list endpoint via the shared client.
func listAllV3[T any](r *Client, resourceKind string, filter string, pageSize int) ([]T, error) {
	return nutanixweb.ListAllV3[T](&r.web, resourceKind, pageSize, filter)
}

// listAll pages through a v3 list endpoint and returns raw entity maps.
func (r *Client) listAll(resourceKind string, filter string, pageSize int) (entities []map[string]any, err error) {
	return listAllV3[map[string]any](r, resourceKind, filter, pageSize)
}

// listAllV4 pages through a v4 list endpoint via the shared client.
func listAllV4[T any](r *Client, path string, pageSize int) ([]T, error) {
	return nutanixweb.ListAllV4[T](&r.web, path, pageSize)
}

// List all clusters, scoped to the configured clusterUuid (if any).
// Prism Central's own self-registered pseudo-cluster entry is excluded --
// see isPrismCentralCluster.
func (r *Client) listClusters() (entities []clusterEntity, err error) {
	entities, err = listAllV3[clusterEntity](r, "cluster", "", clusterPageSize)
	if err != nil {
		return nil, err
	}
	entities = withoutPrismCentralClusters(entities)
	return filterByMatch(entities, r.prism.ClusterUUID, func(entity clusterEntity) string {
		return entity.Metadata.UUID
	}), nil
}

// List all hosts, scoped to the configured clusterUuid (if any). Hosts
// belonging to Prism Central's own pseudo-cluster (i.e. its underlying
// appliance, not a real hypervisor node) are excluded.
func (r *Client) listHosts() (entities []hostEntity, err error) {
	entities, err = listAllV3[hostEntity](r, "host", "", hostPageSize)
	if err != nil {
		return nil, err
	}
	clusters, err := listAllV3[clusterEntity](r, "cluster", "", clusterPageSize)
	if err != nil {
		return nil, err
	}
	entities = excludeHostsByCluster(entities, excludedClusterUUIDs(clusters))
	return filterByMatch(entities, r.prism.ClusterUUID, func(entity hostEntity) string {
		return entity.clusterUUID()
	}), nil
}

// List all VMs, scoped to the configured clusterUuid (if any).
func (r *Client) listVMs() (entities []vmEntity, err error) {
	entities, err = listAllV3[vmEntity](r, "vm", "", vmPageSize)
	if err != nil {
		return nil, err
	}
	return filterByMatch(entities, r.prism.ClusterUUID, func(entity vmEntity) string {
		return entity.Spec.ClusterReference.UUID
	}), nil
}

// List all subnets (networks), scoped to the configured clusterUuid (if any).
func (r *Client) listSubnets() (entities []networkEntity, err error) {
	entities, err = listAllV3[networkEntity](r, "subnet", "", subnetPageSize)
	if err != nil {
		return nil, err
	}
	return filterByMatch(entities, r.prism.ClusterUUID, func(entity networkEntity) string {
		return entity.clusterUUID()
	}), nil
}
