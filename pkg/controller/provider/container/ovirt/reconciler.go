package ovirt

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
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
	// Refresh interval.
	RefreshInterval = 10 * time.Second
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
	// load() completed.
	loaded bool
	// REST client.
	client *Client
	// cancel function.
	cancel func()
	// Last event ID.
	lastEvent int
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
	ctx := context.Background()
	ctx, r.cancel = context.WithCancel(ctx)
	watchList := []*libmodel.Watch{}
	start := func() {
		defer func() {
			for _, w := range watchList {
				w.End()
			}
		}()
	try:
		for {
			select {
			case <-ctx.Done():
				break try
			default:
				if r.loaded {
					err := r.refresh()
					if err != nil {
						r.log.Error(err, "Refresh failed.")
						r.parity = false
					} else {
						r.parity = true
					}
				} else {
					err := r.noteLastEvent()
					if err != nil {
						r.log.Error(err, "Mark last event failed.")
					}
					err = r.load()
					if err == nil {
						watchList = r.watch()
					} else {
						r.log.Error(err, "Load failed.")
					}
				}
				time.Sleep(RefreshInterval)
			}
		}
	}

	go start()

	return nil
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
		"Last event marked.",
		"id",
		r.lastEvent)

	return
}

//
// Load the inventory.
func (r *Reconciler) load() (err error) {
	err = r.connect()
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

	mark := time.Now()

	for _, adapter := range adapterList {
		itr, aErr := adapter.List(r.client)
		if aErr != nil {
			err = aErr
			return
		}
		for {
			object, hasNext := itr.Next()
			if !hasNext {
				break
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

	}
	err = tx.Commit()
	if err == nil {
		r.parity = true
		r.loaded = true
	} else {
		return
	}

	r.log.Info(
		"Initial Parity.",
		"duration",
		time.Since(mark))

	return
}

//
// Add model watches.
func (r *Reconciler) watch() (list []*libmodel.Watch) {
	// Cluster
	w, err := r.db.Watch(
		&model.Cluster{},
		&ClusterEventHandler{
			DB:  r.db,
			log: r.log,
		})
	if err != nil {
		r.log.Error(
			err,
			"create (cluster) watch failed.")
	} else {
		list = append(list, w)
	}
	// Host
	w, err = r.db.Watch(
		&model.Host{},
		&HostEventHandler{
			DB:  r.db,
			log: r.log,
		})
	if err != nil {
		r.log.Error(
			err,
			"create (host) watch failed.")
	} else {
		list = append(list, w)
	}
	// VM
	w, err = r.db.Watch(
		&model.VM{},
		&VMEventHandler{
			Provider: r.provider,
			DB:       r.db,
			log:      r.log,
		})
	if err != nil {
		r.log.Error(
			err,
			"create (VM) watch failed.")
	} else {
		list = append(list, w)
	}
	// NICProfile
	w, err = r.db.Watch(
		&model.NICProfile{},
		&NICProfileHandler{
			DB:  r.db,
			log: r.log,
		})
	if err != nil {
		r.log.Error(
			err,
			"create (NICProfile) watch failed.")
	} else {
		list = append(list, w)
	}
	// DiskProfile
	w, err = r.db.Watch(
		&model.DiskProfile{},
		&DiskProfileHandler{
			DB:  r.db,
			log: r.log,
		})
	if err != nil {
		r.log.Error(
			err,
			"create (DiskProfile) watch failed.")
	} else {
		list = append(list, w)
	}

	return
}

//
// Refresh the inventory.
//  - List events.
//  - Build the changeSet.
//  - Apply the changeSet.
// The two-phased approach ensures we do not hold the
// DB transaction while using the provider API which
// can block or be slow.
func (r *Reconciler) refresh() (err error) {
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
		changeSet, err = r.changeSet(event)
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
func (r *Reconciler) changeSet(event *Event) (list []Updater, err error) {
	for _, adapter := range adapterMap[event.code()] {
		u, aErr := adapter.Apply(event, r.client)
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
