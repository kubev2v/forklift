package nutanix

import (
	"context"
	"net/http"
	liburl "net/url"
	libpath "path"
	"time"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/nutanix"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Settings
const (
	// Retry interval.
	RetryInterval = 5 * time.Second
	// Refresh interval.
	RefreshInterval = 10 * time.Second
	// Default timeout for the HTTP client
	DefaultClientTimeout = 30 * time.Minute
)

// Phases
const (
	Started = ""
	Load    = "load"
	Loaded  = "loaded"
	Parity  = "parity"
	Refresh = "refresh"
)

// Nutanix data collector.
type Collector struct {
	// Provider
	provider *api.Provider
	// DB client.
	db libmodel.DB
	// Logger.
	log logging.LevelLogger
	// has parity.
	parity bool
	// REST client.
	client *Client
	// cancel function.
	cancel func()
	// Phase
	phase string
	// Context
	ctx context.Context
}

// New collector.
func New(db libmodel.DB, provider *api.Provider, secret *core.Secret) (r *Collector) {
	log := logging.WithName("collector|nutanix").WithValues(
		"provider",
		libpath.Join(
			provider.GetNamespace(),
			provider.GetName()))
	clientLog := logging.WithName("client|nutanix").WithValues(
		"provider",
		libpath.Join(
			provider.GetNamespace(),
			provider.GetName()))

	var err error
	clientTimeout := DefaultClientTimeout
	if timeout, ok := provider.Spec.Settings["nutanixClientTimeout"]; ok {
		if clientTimeout, err = time.ParseDuration(timeout); err != nil {
			log.Error(err, "Couldn't parse timeout, falling back to default")
			clientTimeout = DefaultClientTimeout
		}
	}
	r = &Collector{
		client: &Client{
			url:           provider.Spec.URL,
			secret:        secret,
			settings:      provider.Spec.Settings,
			log:           clientLog,
			clientTimeout: clientTimeout,
		},
		provider: provider,
		db:       db,
		log:      log,
	}

	return
}

// The name.
func (r *Collector) Name() string {
	url, err := liburl.Parse(r.client.url)
	if err == nil {
		return url.Host
	}

	return r.client.url
}

// The owner.
func (r *Collector) Owner() meta.Object {
	return r.provider
}

// Get the DB.
func (r *Collector) DB() libmodel.DB {
	return r.db
}

// Reset.
func (r *Collector) Reset() {
	r.parity = false
}

// Has parity.
func (r *Collector) HasParity() bool {
	return r.parity
}

// Follow link.
// Not applicable for Nutanix - the API returns complete objects.
func (r *Collector) Follow(moRef interface{}, p []string, dst interface{}) error {
	return liberr.New("not implemented")
}

// Version - not implemented for Nutanix.
func (r *Collector) Version() (_, _, _, _ string, err error) {
	return
}

// Test connect/logout.
func (r *Collector) Test() (status int, err error) {
	status, err = r.client.connect()
	if err != nil {
		return
	}
	if status != http.StatusOK {
		err = liberr.New("connection test failed", "status", status)
	}

	return
}

// Start the collector.
func (r *Collector) Start() error {
	r.ctx, r.cancel = context.WithCancel(context.Background())
	start := func() {
		defer func() {
			r.log.Info("Stopped.")
		}()
		for {
			if !r.canceled() {
				_ = r.run()
			} else {
				return
			}
		}
	}

	go start()

	return nil
}

// Check if canceled.
func (r *Collector) canceled() (done bool) {
	select {
	case <-r.ctx.Done():
		done = true
	default:
	}

	return
}

// Run the current phase.
func (r *Collector) run() (err error) {
	r.log.V(3).Info(
		"Running.",
		"phase",
		r.phase)
	switch r.phase {
	case Started:
		r.phase = Load
	case Load:
		err = r.collect()
		if err == nil {
			r.phase = Loaded
			r.log.Info("Initial load completed.")
		}
	case Loaded:
		r.phase = Parity
	case Parity:
		r.phase = Refresh
		r.parity = true
		r.log.Info("Initial parity achieved.")
	case Refresh:
		err = r.collect()
		if err == nil {
			r.parity = true
			time.Sleep(RefreshInterval)
		} else {
			r.parity = false
		}
	default:
		err = liberr.New("Phase unknown.")
	}
	if err != nil {
		r.log.Error(
			err,
			"Failed.",
			"phase",
			r.phase)
		time.Sleep(RetryInterval)
	}

	return
}

// Shutdown the collector.
func (r *Collector) Shutdown() {
	r.log.Info("Shutdown.")
	if r.cancel != nil {
		r.cancel()
	}
}

// Collect all inventory resources.
func (r *Collector) collect() (err error) {
	mark := time.Now()

	// Collect all resources
	err = r.clusters()
	if err != nil {
		return
	}
	if r.canceled() {
		return
	}

	err = r.hosts()
	if err != nil {
		return
	}
	if r.canceled() {
		return
	}

	err = r.networks()
	if err != nil {
		return
	}
	if r.canceled() {
		return
	}

	err = r.storageContainers()
	if err != nil {
		return
	}
	if r.canceled() {
		return
	}

	err = r.images()
	if err != nil {
		return
	}
	if r.canceled() {
		return
	}

	err = r.vms()
	if err != nil {
		return
	}
	if r.canceled() {
		return
	}

	r.log.V(3).Info(
		"Collection completed.",
		"duration",
		time.Since(mark))

	return
}

// Collect clusters.
func (r *Collector) clusters() (err error) {
	r.log.V(3).Info("Collecting clusters.")

	entities, err := r.client.listClusters()
	if err != nil {
		return
	}

	tx, err := r.db.Begin()
	if err != nil {
		return
	}
	defer func() {
		_ = tx.End()
	}()

	for _, entity := range entities {
		if r.canceled() {
			return
		}
		m := &model.Cluster{}
		applyCluster(entity, m)
		err = tx.Insert(m)
		if err != nil {
			return
		}
	}

	err = tx.Commit()
	if err != nil {
		return
	}

	r.log.V(3).Info("Clusters collected.", "count", len(entities))

	return
}

// Collect hosts.
func (r *Collector) hosts() (err error) {
	r.log.V(3).Info("Collecting hosts.")

	entities, err := r.client.listHosts()
	if err != nil {
		return
	}

	tx, err := r.db.Begin()
	if err != nil {
		return
	}
	defer func() {
		_ = tx.End()
	}()

	for _, entity := range entities {
		if r.canceled() {
			return
		}
		m := &model.Host{}
		applyHost(entity, m)
		err = tx.Insert(m)
		if err != nil {
			return
		}
	}

	err = tx.Commit()
	if err != nil {
		return
	}

	r.log.V(3).Info("Hosts collected.", "count", len(entities))

	return
}

// Collect networks.
func (r *Collector) networks() (err error) {
	r.log.V(3).Info("Collecting networks.")

	entities, err := r.client.listSubnets()
	if err != nil {
		return
	}

	tx, err := r.db.Begin()
	if err != nil {
		return
	}
	defer func() {
		_ = tx.End()
	}()

	for _, entity := range entities {
		if r.canceled() {
			return
		}
		m := &model.Network{}
		applyNetwork(entity, m)
		err = tx.Insert(m)
		if err != nil {
			return
		}
	}

	err = tx.Commit()
	if err != nil {
		return
	}

	r.log.V(3).Info("Networks collected.", "count", len(entities))

	return
}

// Collect storage containers.
func (r *Collector) storageContainers() (err error) {
	r.log.V(3).Info("Collecting storage containers.")

	entities, err := r.client.listStorageContainers()
	if err != nil {
		r.log.Error(err, "Storage container collection failed; continuing without storage inventory")
		return nil
	}

	tx, err := r.db.Begin()
	if err != nil {
		return
	}
	defer func() {
		_ = tx.End()
	}()

	for _, entity := range entities {
		if r.canceled() {
			return
		}
		m := &model.StorageContainer{}
		applyStorageContainer(entity, m)
		err = tx.Insert(m)
		if err != nil {
			return
		}
	}

	err = tx.Commit()
	if err != nil {
		return
	}

	r.log.V(3).Info("Storage containers collected.", "count", len(entities))

	return
}

// Collect images.
func (r *Collector) images() (err error) {
	r.log.V(3).Info("Collecting images.")

	entities, err := r.client.listImages()
	if err != nil {
		return
	}

	tx, err := r.db.Begin()
	if err != nil {
		return
	}
	defer func() {
		_ = tx.End()
	}()

	for _, entity := range entities {
		if r.canceled() {
			return
		}
		m := &model.Image{}
		applyImage(entity, m)
		err = tx.Insert(m)
		if err != nil {
			return
		}
	}

	err = tx.Commit()
	if err != nil {
		return
	}

	r.log.V(3).Info("Images collected.", "count", len(entities))

	return
}

// Collect VMs.
func (r *Collector) vms() (err error) {
	r.log.V(3).Info("Collecting VMs.")

	entities, err := r.client.listVMs()
	if err != nil {
		return
	}

	storageNames, networkNames, err := r.vmLookupMaps()
	if err != nil {
		return
	}

	tx, err := r.db.Begin()
	if err != nil {
		return
	}
	defer func() {
		_ = tx.End()
	}()

	for _, entity := range entities {
		if r.canceled() {
			return
		}
		m := &model.VM{}
		applyVM(entity, m)
		enrichVM(m, storageNames, networkNames)
		err = tx.Insert(m)
		if err != nil {
			return
		}
	}

	err = tx.Commit()
	if err != nil {
		return
	}

	r.log.V(3).Info("VMs collected.", "count", len(entities))

	return
}

func (r *Collector) vmLookupMaps() (storageNames, networkNames map[string]string, err error) {
	storageNames = map[string]string{}
	networkNames = map[string]string{}

	storageList := []model.StorageContainer{}
	err = r.db.List(&storageList, libmodel.ListOptions{Detail: model.MaxDetail})
	if err != nil {
		return
	}
	for _, sc := range storageList {
		storageNames[sc.ID] = sc.Name
	}

	networkList := []model.Network{}
	err = r.db.List(&networkList, libmodel.ListOptions{Detail: model.MaxDetail})
	if err != nil {
		return
	}
	for _, network := range networkList {
		networkNames[network.ID] = network.Name
	}

	return
}
