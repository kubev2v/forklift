package ovirt

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	fb "github.com/konveyor/controller/pkg/filebacked"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	libweb "github.com/konveyor/controller/pkg/inventory/web"
	"github.com/konveyor/controller/pkg/logging"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
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
	// Event page (size).
	EventPage = 10
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
	// Event state.
	event struct {
		id   int
		page int
	}
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

	r.event.page = 1

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
	start := func() {
	try:
		for {
			select {
			case <-ctx.Done():
				break try
			default:
				if r.parity {
					err := r.refresh()
					if err != nil {
						r.log.Error(err, "Refresh failed.")
					}
				} else {
					err := r.drainEvent()
					if err != nil {
						r.log.Error(err, "Drain (event) failed.")
					}
					err = r.load()
					if err != nil {
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
// Drain events.
func (r *Reconciler) drainEvent() (err error) {
	if r.event.id > 0 {
		return
	}
	defer func() {
		if err != nil {
			r.event.page = 1
			r.event.id = 0
		}
	}()
	err = r.connect()
	if err != nil {
		return
	}
	for {
		itr, lErr := r.listEvent()
		if lErr != nil || itr.Len() == 0 {
			err = lErr
			break
		} else {
			time.Sleep(100 * time.Millisecond)
		}
	}

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
			object, hasNext, nErr := itr.Next()
			if nErr != nil {
				err = nErr
				return
			}
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
// Refresh the inventory.
func (r *Reconciler) refresh() (err error) {
	err = r.connect()
	if err != nil {
		return
	}
	itr, err := r.listEvent()
	if err != nil {
		return
	}
	tx, err := r.db.Begin()
	if err != nil {
		return
	}
	defer func() {
		if err == nil {
			_ = tx.Commit()
		} else {
			_ = tx.End()
		}
	}()
	for {
		event := &Event{}
		hasNext, nErr := itr.NextWith(event)
		if nErr != nil {
			err = nErr
			return
		}
		if !hasNext {
			break
		}
		r.log.V(3).Info("Event received.",
			"event",
			event)
		if adapter, found := adapterMap[event.code()]; found {
			err = adapter.Apply(r.client, tx, event)
			if err == nil {
				r.log.V(3).Info(
					"Event applied.",
					"event",
					event.Code)
			} else {
				r.log.Error(
					err,
					"Apply event failed.",
					"event",
					event)
				err = nil
			}
		} else {
			r.log.Info(
				"Event not mapped to adapter.",
				"event",
				event)
		}
	}

	return
}

//
// List Event collection.
// Query by list of event types and date.
func (r *Reconciler) listEvent() (itr fb.Iterator, err error) {
	eventList := EventList{}
	codes := []string{}
	for n, _ := range adapterMap {
		codes = append(codes, fmt.Sprintf("type=%d", n))
	}
	eventQ := strings.Join(
		[]string{
			fmt.Sprintf("date>%s", time.Now().Format("01/02/2006")),
			strings.Join(codes, " or "),
		},
		" and ")
	search := fmt.Sprintf(
		"%s sortby time asc page %d",
		eventQ,
		r.event.page)
	err = r.client.list(
		"events",
		&eventList,
		libweb.Param{
			Key:   "search",
			Value: search,
		},
		libweb.Param{
			Key:   "max",
			Value: strconv.Itoa(EventPage),
		})
	if err != nil {
		return
	}
	if len(eventList.Items) == EventPage {
		r.event.page++
	}
	list := fb.NewList()
	for _, e := range eventList.Items {
		if e.id() <= r.event.id {
			continue
		}
		r.event.id = e.id()
		err = list.Append(e)
		if err != nil {
			return
		}
	}

	itr = list.Iter()

	return
}

//
// Connect.
func (r *Reconciler) connect() error {
	return r.client.connect()
}
