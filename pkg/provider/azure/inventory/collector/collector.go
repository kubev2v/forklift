package collector

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	libcontainer "github.com/kubev2v/forklift/pkg/lib/inventory/container"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	azureclient "github.com/kubev2v/forklift/pkg/provider/azure/inventory/client"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Collector struct {
	libcontainer.Collector
	db         libmodel.DB
	provider   *api.Provider
	secret     *core.Secret
	client     *azureclient.Client
	log        logging.LevelLogger
	ctx        context.Context
	cancel     context.CancelFunc
	done       chan struct{}
	parity     atomic.Bool
	collecting bool
	mutex      sync.Mutex
}

func New(db libmodel.DB, provider *api.Provider, secret *core.Secret) libcontainer.Collector {
	log := logging.WithName("collector|azure").WithValues(
		"namespace", provider.GetNamespace(),
		"name", provider.GetName())

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
	return r.parity.Load()
}

func (r *Collector) Start() error {
	client, err := azureclient.New(r.provider, r.secret)
	if err != nil {
		return err
	}
	r.client = client

	ctx := context.Background()
	r.ctx, r.cancel = context.WithCancel(ctx)
	r.done = make(chan struct{})

	start := func() {
		defer close(r.done)
		defer func() {
			r.log.Info("Collection loop stopped.")
		}()

		if err := r.Collect(); err != nil {
			r.log.Error(err, "Initial collection failed")
		} else {
			r.parity.Store(true)
			r.log.Info("Initial collection completed, parity achieved.")
		}

		ticker := time.NewTicker(RefreshInterval)
		defer ticker.Stop()

		for {
			select {
			case <-r.ctx.Done():
				return
			case <-ticker.C:
				r.log.V(1).Info("Starting periodic collection")
				if err := r.Collect(); err != nil {
					r.log.Error(err, "Periodic collection failed")
					r.parity.Store(false)
				} else {
					r.parity.Store(true)
					r.log.V(1).Info("Periodic collection completed")
				}
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
	if r.done != nil {
		<-r.done
	}
	r.log.Info("Collector shut down.")
}

func (r *Collector) Reset() {
	r.parity.Store(false)
	r.log.Info("Collector reset.")
}

type collectionTask struct {
	name string
	fn   func(context.Context) error
}

func (r *Collector) Collect() error {
	r.mutex.Lock()
	if r.collecting {
		r.mutex.Unlock()
		r.log.Info("Collection already in progress, skipping")
		return nil
	}
	r.collecting = true
	r.mutex.Unlock()

	defer func() {
		r.mutex.Lock()
		r.collecting = false
		r.mutex.Unlock()
	}()

	ctx := r.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	r.log.V(1).Info("Starting collection")

	tasks := []collectionTask{
		{"disks", r.collectDisks},
		{"vms", r.collectVMs},
		{"networks", r.collectNetworks},
		{"diskTypes", r.collectDiskTypes},
	}

	var errs []error
	successCount := 0

	for _, task := range tasks {
		if err := task.fn(ctx); err != nil {
			r.log.Error(err, "Failed to collect "+task.name)
			errs = append(errs, fmt.Errorf("%s: %w", task.name, err))
		} else {
			successCount++
		}
	}

	if len(errs) == 0 {
		r.log.V(1).Info("Collection completed successfully", "total", len(tasks))
		return nil
	}

	if len(errs) == len(tasks) {
		r.log.Error(nil, "All collections failed")
		return errors.Join(errs...)
	}

	r.log.Info("Collection partially completed",
		"successful", successCount,
		"failed", len(errs),
		"total", len(tasks))
	r.log.V(2).Info("Partial collection errors", "errors", errors.Join(errs...))
	return nil
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := azureclient.New(r.provider, r.secret)
	if err != nil {
		status = 400
		return
	}

	_, err = client.ListVirtualMachines(ctx)
	if err != nil {
		status = 500
		return
	}

	status = 200
	return
}
