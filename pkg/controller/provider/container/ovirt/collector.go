package ovirt

import (
	"context"
	"fmt"
	"net/http"
	liburl "net/url"
	libpath "path"
	"strconv"
	"strings"
	"time"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/ovirt"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
	libweb "github.com/kubev2v/forklift/pkg/lib/inventory/web"
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

// oVirt data collector.
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
	// Last event ID.
	lastEvent int
	// Phase
	phase string
	// List of watches.
	watches []*libmodel.Watch
}

// New collector.
func New(db libmodel.DB, provider *api.Provider, secret *core.Secret) (r *Collector) {
	log := logging.WithName("collector|ovirt").WithValues(
		"provider",
		libpath.Join(
			provider.GetNamespace(),
			provider.GetName()))
	clientLog := logging.WithName("client|ovirt").WithValues(
		"provider",
		libpath.Join(
			provider.GetNamespace(),
			provider.GetName()))

	var err error
	clientTimeout := DefaultClientTimeout
	if timeout, ok := provider.Spec.Settings["ovirtClientTimeout"]; ok {
		if clientTimeout, err = time.ParseDuration(timeout); err != nil {
			log.Error(err, "Couldn't parse timeout, falling back to default")
			clientTimeout = DefaultClientTimeout
		}
	}
	r = &Collector{
		client: &Client{
			url:           provider.Spec.URL,
			secret:        secret,
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

// Reset.
func (r *Collector) HasParity() bool {
	return r.parity
}

// Follow link
func (r *Collector) Follow(moRef interface{}, p []string, dst interface{}) error {
	return fmt.Errorf("not implemented")
}

// Test connect/logout.
func (r *Collector) Test() (status int, err error) {
	_, status, err = r.client.system()
	if status != http.StatusOK {
		err = liberr.New("got status != 200 from oVirt", "status", status)
	}

	return
}

func (r *Collector) Version() (major, minor, build, revision string, err error) {
	system, _, err := r.client.system()
	if err != nil {
		return
	}
	major, minor, build, revision = parseVersion(system.Product.Version.FullVersion)
	return
}

func parseVersion(fullVersion string) (major, minor, build, revision string) {
	version := strings.Split(fullVersion, ".")
	major = version[0]
	minor = version[1]

	split := strings.SplitN(version[2], "-", 2)
	build = split[0]
	switch {
	case len(split) > 1:
		revision = split[1]
	case len(version) > 3:
		revision = strings.Split(version[3], "-")[0]
	default:
		revision = "0"
	}
	return
}

// Start the collector.
func (r *Collector) Start() error {
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

// Run the current phase.
func (r *Collector) run(ctx *Context) (err error) {
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

// Shutdown the collector.
func (r *Collector) Shutdown() {
	r.log.Info("Shutdown.")
	if r.cancel != nil {
		r.cancel()
	}
}

// Fetch and note that last event.
func (r *Collector) noteLastEvent() (err error) {
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

// Load the inventory.
func (r *Collector) load(ctx *Context) (err error) {
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

// End watches.
func (r *Collector) endWatch() {
	for _, watch := range r.watches {
		watch.End()
	}
}

// Refresh the inventory.
//   - List events.
//   - Build the changeSet.
//   - Apply the changeSet.
//
// The two-phased approach ensures we do not hold the
// DB transaction while using the provider API which
// can block or be slow.
func (r *Collector) refresh(ctx *Context) (err error) {
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
		if err != nil {
			r.log.Error(
				err,
				"Getting the changeset failed",
				"event",
				event)
			continue
		}
		err = r.apply(changeSet)
		if err != nil {
			r.log.Error(
				err,
				"Apply changeSet failed.",
				"event",
				event)

			continue
		}

		r.log.V(3).Info(
			"Event applied.",
			"event",
			event)
	}

	return
}

// Build the changeSet.
func (r *Collector) changeSet(ctx *Context, event *Event) (list []Updater, err error) {
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

// List Event collection.
// Query by list of event types since lastEvent (marked).
func (r *Collector) listEvent() (list []Event, err error) {
	eventList := EventList{}
	codes := []string{}
	for n := range adapterMap {
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

// Connect.
func (r *Collector) connect() error {
	_, err := r.client.connect()
	return err
}
