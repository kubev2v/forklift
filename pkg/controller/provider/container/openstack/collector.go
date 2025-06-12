package openstack

import (
	"context"
	"fmt"
	liburl "net/url"
	libpath "path"
	"time"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/openstack"
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
)

// Phases
const (
	Started = ""
	Load    = "load"
	Loaded  = "loaded"
	Parity  = "parity"
	Refresh = "refresh"
)

// Openstack data collector.
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
	// Start Time
	startTime time.Time
	// Phase
	phase string
	// List of watches.
	watches []*libmodel.Watch
}

// New collector.
func New(db libmodel.DB, provider *api.Provider, secret *core.Secret) (r *Collector) {
	log := logging.WithName("collector|openstack").WithValues(
		"provider",
		libpath.Join(
			provider.GetNamespace(),
			provider.GetName()))

	client := &Client{}
	client.URL = provider.Spec.URL
	client.Log = log
	client.LoadOptionsFromSecret(secret)

	r = &Collector{
		client:   client,
		provider: provider,
		db:       db,
		log:      log,
	}

	return
}

// The name.
func (r *Collector) Name() string {
	url, err := liburl.Parse(r.client.URL)
	if err == nil {
		return url.Host
	}

	return r.client.URL
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

// Reset.
func (r *Collector) HasParity() bool {
	return r.parity
}

// Test connect/logout.
func (r *Collector) Test() (_ int, err error) {
	err = r.client.Authenticate()
	return
}

// NO-OP
func (r *Collector) Version() (_, _, _, _ string, err error) {
	return
}

// Follow link
func (r *Collector) Follow(moRef interface{}, p []string, dst interface{}) error {
	return fmt.Errorf("not implemented")
}

// Start the collector.
func (r *Collector) Start() error {
	ctx := Context{
		client: r.client,
		db:     r.db,
		log:    r.log,
	}
	ctx.ctx, r.cancel = context.WithCancel(context.Background())
	start := func() {
		defer func() {
			r.endWatch()
			r.log.Info("Stopped.")
		}()
		for {
			if !ctx.canceled() {
				_ = r.run(&ctx)
			} else {
				return
			}
		}
	}

	go start()

	return nil
}

// Run the current phase.
func (r *Collector) run(ctx *Context) (err error) {
	r.log.V(3).Info(
		"Running.",
		"phase",
		r.phase)
	switch r.phase {
	case Started:
		err = r.client.Connect()
		if err != nil {
			return
		}
		r.startTime = time.Now()
		r.phase = Load
	case Load:
		err = r.load(ctx)
		if err == nil {
			r.phase = Loaded
		}
	case Loaded:
		err = r.refresh(ctx)
		if err == nil {
			r.phase = Parity
		}
	case Parity:
		r.endWatch()
		err = r.beginWatch()
		if err == nil {
			r.phase = Refresh
			r.parity = true
		}
	case Refresh:
		err = r.refresh(ctx)
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

// Load the inventory.
func (r *Collector) load(ctx *Context) (err error) {
	mark := time.Now()
	for _, adapter := range adapterList {
		if ctx.canceled() {
			return
		}
		err = r.create(ctx, adapter)
		if err != nil {
			return
		}
	}
	r.log.Info(
		"Initial Parity.",
		"duration",
		time.Since(mark))

	return
}

// List and create resources using the adapter.
func (r *Collector) create(ctx *Context, adapter Adapter) (err error) {
	itr, aErr := adapter.List(ctx)

	if aErr != nil {
		err = aErr
		return
	}
	tx, err := r.db.Begin()
	if err != nil {
		return
	}
	defer func() {
		_ = tx.End()
	}()
	for {
		object, hasNext := itr.Next()
		if !hasNext {
			break
		}
		if ctx.canceled() {
			return
		}
		m := object.(libmodel.Model)
		err = tx.Insert(m)
		if err != nil {
			return
		}
	}
	err = tx.Commit()
	if err != nil {
		return
	}

	return
}

// Add model watches.
func (r *Collector) beginWatch() (err error) {
	defer func() {
		if err != nil {
			r.endWatch()
		}
	}()
	// Cluster
	w, err := r.db.Watch(
		&model.VM{},
		&VMEventHandler{
			Provider: r.provider,
			DB:       r.db,
			log:      r.log,
		})

	if err == nil {
		r.watches = append(r.watches, w)
	} else {
		return
	}
	return
}

// End watches.
func (r *Collector) endWatch() {
	for _, watch := range r.watches {
		watch.End()
	}
}

// Refresh the inventory.
//   - List modified vms.
//   - Build the changeSet.
//   - Apply the changeSet.
//
// The two-phased approach ensures we do not hold the
// DB transaction while using the provider API which
// can block or be slow.
func (r *Collector) refresh(ctx *Context) (err error) {
	var deletes, updates []Updater
	mark := time.Now()
	for _, adapter := range adapterList {
		if ctx.canceled() {
			return
		}
		deletes, err = adapter.DeleteUnexisting(ctx)
		if err != nil {
			return
		}
		err = r.apply(deletes)
		if err != nil {
			return
		}
		updates, err = adapter.GetUpdates(ctx)
		if err != nil {
			return
		}
		err = r.apply(updates)
		if err != nil {
			return
		}
	}
	r.log.Info(
		"Refresh finished.",
		"duration",
		time.Since(mark))
	return
}

// Apply the changeSet.
func (r *Collector) apply(changeSet []Updater) (err error) {
	tx, err := r.db.Begin()
	if err != nil {
		return
	}
	defer func() {
		_ = tx.End()
	}()
	for _, updater := range changeSet {
		err = updater(tx)
		if err != nil {
			return
		}
	}
	err = tx.Commit()
	return
}
