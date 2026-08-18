package collector

import (
	"context"
	"path"
	"sync"
	"time"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	libcontainer "github.com/kubev2v/forklift/pkg/lib/inventory/container"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ libcontainer.Collector = &Collector{}

type Collector struct {
	libcontainer.Collector
	db         libmodel.DB
	provider   *api.Provider
	secret     *core.Secret
	log        logging.LevelLogger
	cancel     context.CancelFunc
	parity     bool
	collecting bool
	mutex      sync.Mutex
}

func New(db libmodel.DB, provider *api.Provider, secret *core.Secret) libcontainer.Collector {
	log := logging.WithName("collector|azure").WithValues(
		"provider",
		path.Join(
			provider.GetNamespace(),
			provider.GetName()))

	return &Collector{
		db:       db,
		provider: provider,
		secret:   secret,
		log:      log,
	}
}

func (r *Collector) Name() string {
	return "Azure"
}

func (r *Collector) HasParity() bool {
	return r.parity
}

func (r *Collector) Start() error {
	ctx := context.Background()
	ctx, r.cancel = context.WithCancel(ctx)

	start := func() {
		defer func() {
			r.log.Info("Collection loop stopped.")
		}()

		r.parity = true
		r.log.Info("Initial collection completed (noop), parity achieved.")

		ticker := time.NewTicker(RefreshInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.log.V(1).Info("Periodic collection (noop)")
				r.parity = true
			}
		}
	}

	go start()

	r.log.Info("Collector started.")
	return nil
}

func (r *Collector) Shutdown() {
	if r.cancel != nil {
		r.cancel()
	}
	r.log.Info("Collector shut down.")
}

func (r *Collector) Reset() {
	r.parity = false
	r.log.Info("Collector reset.")
}

func (r *Collector) DB() libmodel.DB {
	return r.db
}

func (r *Collector) Version() (version, product, apiVersion, instanceUuid string, err error) {
	version = "azure"
	product = "Microsoft Azure"
	return
}

func (r *Collector) Owner() meta.Object {
	return r.provider
}

func (r *Collector) Test() (status int, err error) {
	status = 200
	return
}
