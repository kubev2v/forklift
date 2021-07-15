package ovirt

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	liberr "github.com/konveyor/controller/pkg/error"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	libweb "github.com/konveyor/controller/pkg/inventory/web"
	"github.com/konveyor/controller/pkg/logging"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/ovirt"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	liburl "net/url"
	libpath "path"
	"strconv"
	"strings"
	"time"
)

//
// Settings
const (
	// Retry interval.
	RetryInterval = 5 * time.Second
	// Refresh interval.
	RefreshInterval = 10 * time.Second
)

//
// Phases
const (
	Started = ""
	Load    = "load"
	Loaded  = "loaded"
	Parity  = "parity"
	Refresh = "refresh"
)

//
// oVirt data reconciler.
type Reconciler struct {
	// Provider
	provider *api.Provider
	// DB client.
	db libmodel.DB
	// Logger.
	log logr.Logger
	// has parity.
	parity bool
	// REST client.
	client *Client
	// cancel function.
	cancel func()
	// Last event ID.
	lastEvent int
	// Phase
	phase string
	// List of watches.
	watches []*libmodel.Watch
}

//
// New reconciler.
func New(db libmodel.DB, provider *api.Provider, secret *core.Secret) (r *Reconciler) {
	log := logging.WithName("reconciler|ovirt").WithValues(
		"provider",
		libpath.Join(
			provider.GetNamespace(),
			provider.GetName()))
	r = &Reconciler{
		client: &Client{
			url:    provider.Spec.URL,
			secret: secret,
		},
		provider: provider,
		db:       db,
		log:      log,
	}

	return
}

//
// The name.
func (r *Reconciler) Name() string {
	url, err := liburl.Parse(r.client.url)
	if err == nil {
		return url.Host
	}

	return r.client.url
}

//
// The owner.
func (r *Reconciler) Owner() meta.Object {
	return r.provider
}

//
// Get the DB.
func (r *Reconciler) DB() libmodel.DB {
	return r.db
}

//
// Reset.
func (r *Reconciler) Reset() {
	r.parity = false
}

//
// Reset.
func (r *Reconciler) HasParity() bool {
	return r.parity
}

//
// Test connect/logout.
func (r *Reconciler) Test() (err error) {
	_, err = r.client.system()
	return
}

//
// Start the reconciler.
func (r *Reconciler) Start() error {
	ctx := Context{
		client: r.client,
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

//
// Run the current phase.
func (r *Reconciler) run(ctx *Context) (err error) {
	r.log.V(3).Info(
		"Running.",
		"phase",
		r.phase)
	switch r.phase {
	case Started:
		err = r.noteLastEvent()
		if err == nil {
			r.phase = Load
		}
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

//
// Shutdown the reconciler.
func (r *Reconciler) Shutdown() {
	r.log.Info("Shutdown.")
	if r.cancel != nil {
		r.cancel()
	}
}

//
// Fetch and note that last event.
func (r *Reconciler) noteLastEvent() (err error) {
	err = r.connect()
	if err != nil {
		return
	}
	eventList := EventList{}
	err = r.client.list(
		"events",
		&eventList,
		libweb.Param{
			Key:   "max",
			Value: "1",
		})
	if err != nil {
		return
	}
	if len(eventList.Items) > 0 {
		r.lastEvent = eventList.Items[0].id()
	}

	r.log.Info(
		"Last event noted.",
		"id",
		r.lastEvent)

	return
}

//
// Load the inventory.
func (r *Reconciler) load(ctx *Context) (err error) {
	err = r.connect()
	if err != nil {
		return
	}
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

//
// List and create resources using the adapter.
func (r *Reconciler) create(ctx *Context, adapter Adapter) (err error) {
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
		r.log.V(3).Info(
			"Model created.",
			"model",
			libmodel.Describe(m))
	}
	err = tx.Commit()
	if err != nil {
		return
	}

	return
}

//
// Add model watches.
func (r *Reconciler) beginWatch() (err error) {
	defer func() {
		if err != nil {
			r.endWatch()
		}
	}()
	// Cluster
	w, err := r.db.Watch(
		&model.Cluster{},
		&ClusterEventHandler{
			DB:  r.db,
			log: r.log,
		})
	if err == nil {
		r.watches = append(r.watches, w)
	} else {
		return
	}
	// Host
	w, err = r.db.Watch(
		&model.Host{},
		&HostEventHandler{
			DB:  r.db,
			log: r.log,
		})
	if err == nil {
		r.watches = append(r.watches, w)
	} else {
		return
	}
	// VM
	w, err = r.db.Watch(
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
	// NICProfile
	w, err = r.db.Watch(
		&model.NICProfile{},
		&NICProfileHandler{
			DB:  r.db,
			log: r.log,
		})
	if err == nil {
		r.watches = append(r.watches, w)
	} else {
		return
	}
	// DiskProfile
	w, err = r.db.Watch(
		&model.DiskProfile{},
		&DiskProfileHandler{
			DB:  r.db,
			log: r.log,
		})
	if err == nil {
		r.watches = append(r.watches, w)
	} else {
		return
	}

	return
}

//
// End watches.
func (r *Reconciler) endWatch() {
	for _, watch := range r.watches {
		watch.End()
	}
}

//
// Refresh the inventory.
//  - List events.
//  - Build the changeSet.
//  - Apply the changeSet.
// The two-phased approach ensures we do not hold the
// DB transaction while using the provider API which
// can block or be slow.
func (r *Reconciler) refresh(ctx *Context) (err error) {
	err = r.connect()
	if err != nil {
		return
	}
	list, err := r.listEvent()
	if err != nil {
		return
	}
	for i := range list {
		event := &list[i]
		r.log.V(3).Info("Event received.",
			"event",
			event)
		var changeSet []Updater
		changeSet, err = r.changeSet(ctx, event)
		if err == nil {
			err = r.apply(changeSet)
		}
		if err != nil {
			r.log.Error(
				err,
				"Apply event failed.",
				"event",
				event)
			continue
		}

		r.log.V(3).Info(
			"Event applied.",
			"event",
			event.Code)
	}

	return
}

//
// Build the changeSet.
func (r *Reconciler) changeSet(ctx *Context, event *Event) (list []Updater, err error) {
	for _, adapter := range adapterMap[event.code()] {
		u, aErr := adapter.Apply(ctx, event)
		if aErr != nil {
			err = aErr
			return
		}
		list = append(list, u)
	}
	return
}

//
// Apply the changeSet.
func (r *Reconciler) apply(changeSet []Updater) (err error) {
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

//
// List Event collection.
// Query by list of event types since lastEvent (marked).
func (r *Reconciler) listEvent() (list []Event, err error) {
	eventList := EventList{}
	codes := []string{}
	for n, _ := range adapterMap {
		codes = append(codes, fmt.Sprintf("type=%d", n))
	}
	search := strings.Join(codes, " or ")
	err = r.client.list(
		"events",
		&eventList,
		libweb.Param{
			Key:   "search",
			Value: search,
		},
		libweb.Param{
			Key:   "from",
			Value: strconv.Itoa(r.lastEvent),
		})
	if err != nil {
		return
	}
	if len(eventList.Items) > 0 {
		r.lastEvent = eventList.Items[0].id()
		eventList.sort()
		list = eventList.Items
	}

	r.log.V(1).Info(
		"List event succeeded.",
		"count",
		len(list),
		"last-id",
		r.lastEvent)

	return
}

//
// Connect.
func (r *Reconciler) connect() error {
	return r.client.connect()
}
