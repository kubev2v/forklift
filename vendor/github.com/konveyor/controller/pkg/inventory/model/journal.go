package model

import (
	"fmt"
	"github.com/go-logr/logr"
	liberr "github.com/konveyor/controller/pkg/error"
	fb "github.com/konveyor/controller/pkg/filebacked"
	"github.com/konveyor/controller/pkg/logging"
	"github.com/konveyor/controller/pkg/ref"
	"sync"
)

//
// Serial number pool.
var serial Serial

//
// Event Actions.
var (
	Started uint8 = 0x00
	Parity  uint8 = 0x01
	Error   uint8 = 0x02
	End     uint8 = 0x04
	Created uint8 = 0x10
	Updated uint8 = 0x20
	Deleted uint8 = 0x40
)

//
// Model event.
type Event struct {
	// ID.
	ID uint64
	// Labels.
	Labels []string
	// The event subject.
	Model Model
	// The event action (created|updated|deleted).
	Action uint8
	// The updated model.
	Updated Model
}

//
// Get whether the event has the specified label.
func (r *Event) HasLabel(label string) bool {
	for _, l := range r.Labels {
		if l == label {
			return true
		}
	}

	return false
}

//
// String representation.
func (r *Event) String() string {
	action := "unknown"
	switch r.Action {
	case Parity:
		action = "parity"
	case Error:
		action = "error"
	case End:
		action = "end"
	case Created:
		action = "created"
	case Updated:
		action = "updated"
	case Deleted:
		action = "deleted"
	}
	model := ""
	if r.Model != nil {
		model = Describe(r.Model)
	}
	return fmt.Sprintf(
		"event-%.4d: %s model=%s",
		r.ID,
		action,
		model)
}

//
// Append self to the list.
// The protocol is to write:
//   Event (self)
//   Event.Model
//   Event.Updated (optional)
func (r *Event) append(list *fb.List) {
	list.Append(Event{
		ID:     r.ID,
		Labels: r.Labels,
		Action: r.Action,
	})
	list.Append(r.Model)
	if r.Action == Updated {
		list.Append(r.Updated)
	}
}

//
// Read self from the iterator.
// The protocol is to read:
//   Event (self)
//   Event.Model
//   Event.Updated (optional)
func (r *Event) next(itr fb.Iterator) (hasNext bool) {
	hasNext = itr.NextWith(r)
	if !hasNext {
		return
	}
	object, hasNext := itr.Next()
	if hasNext {
		r.Model = object.(Model)
	} else {
		return
	}
	if r.Action == Updated {
		object, hasNext = itr.Next()
		if hasNext {
			r.Updated = object.(Model)
		} else {
			return
		}
	}

	hasNext = true

	return
}

//
// Watch options.
type WatchOptions struct {
	// Initial snapshot.
	// List models and report as `Created` events.
	Snapshot bool
}

//
// Event handler.
type EventHandler interface {
	// Watch options.
	Options() WatchOptions
	// Watch has started.
	Started(watchID uint64)
	// Parity marker.
	// The watch has delivered the initial set
	// of `Created` events.
	Parity()
	// A model has been created.
	Created(Event)
	// A model has been updated.
	Updated(Event)
	// A model has been deleted.
	Deleted(Event)
	// An error has occurred delivering an event.
	Error(error)
	// An event watch has ended.
	End()
}

//
// Model event watch.
type Watch struct {
	// Model to be watched.
	Model Model
	// Event handler.
	Handler EventHandler
	// ID
	id uint64
	// Event queue.
	queue chan fb.Iterator
	// Journal.
	journal *Journal
	// Logger.
	log logr.Logger
	// Started
	started bool
	// Done
	done bool
}

//
// String representation.
func (w *Watch) String() string {
	kind := ref.ToKind(w.Model)
	return fmt.Sprintf(
		"watch-%.4d: model=%s",
		w.id,
		kind)
}

//
// End the watch.
func (w *Watch) End() {
	w.journal.End(w)
}

//
// The watch has not ended.
func (w *Watch) Alive() bool {
	return !w.done
}

//
// Match by model `kind`.
func (w *Watch) Match(model Model) bool {
	return ref.ToKind(w.Model) == ref.ToKind(model)
}

//
// Queue event.
func (w *Watch) notify(itr fb.Iterator) {
	defer func() {
		recover()
	}()
	select {
	case w.queue <- itr:
	default:
		description := "full queue, event discarded"
		w.Handler.Error(liberr.New(description))
		w.log.V(3).Info(description)
	}
}

//
// Run the watch.
// Forward events to the `handler`.
func (w *Watch) Start(snapshot fb.Iterator) {
	if w.started {
		return
	}
	w.log.V(3).Info("watch started.")
	w.Handler.Started(w.id)
	run := func() {
		defer func() {
			w.started = false
			w.done = true
			w.Handler.End()
			w.log.V(3).Info("watch stopped.")
		}()
		for {
			m, hasNext := snapshot.Next()
			if hasNext {
				w.Handler.Created(
					Event{
						Action: Created,
						Model:  m.(Model),
					})
			} else {
				w.log.V(3).Info("has parity.")
				w.Handler.Parity()
				break
			}
		}
		for itr := range w.queue {
			for {
				event := Event{}
				hasNext := event.next(itr)
				if !hasNext {
					break
				}
				if !w.Match(event.Model) {
					continue
				}
				w.log.V(5).Info(
					"event received.",
					"event",
					event.String())
				switch event.Action {
				case Created:
					w.Handler.Created(event)
				case Updated:
					w.Handler.Updated(event)
				case Deleted:
					w.Handler.Deleted(event)
				default:
					w.log.Info(
						"unknown action.",
						"event",
						event.String())
				}
			}
		}
	}

	w.started = true
	go run()
}

//
// Terminate.
func (w *Watch) terminate() {
	if w.started {
		close(w.queue)
	}
}

//
// DB journal.
// Provides model watch events.
type Journal struct {
	mutex sync.RWMutex
	// Logger.
	log logr.Logger
	// List of registered watches.
	watches []*Watch
}

//
// Watch a `watch` of the specified model.
// The returned watch has not been started.
// See: Watch.Start().
func (r *Journal) Watch(model Model, handler EventHandler) (*Watch, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	id := serial.next(0)
	log := logging.WithName("journal|watch").WithValues(
		"id",
		id,
		"model",
		ref.ToKind(model))
	watch := &Watch{
		Handler: handler,
		Model:   model,
		id:      id,
		journal: r,
		log:     log,
	}
	r.watches = append(r.watches, watch)
	watch.queue = make(chan fb.Iterator, 250)

	r.log.V(3).Info(
		"watch created.",
		"watch",
		watch.String())

	return watch, nil
}

//
// End watch.
func (r *Journal) End(watch *Watch) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	kept := []*Watch{}
	for _, w := range r.watches {
		if w != watch {
			kept = append(kept, w)
			continue
		}
		w.terminate()
		r.log.V(3).Info(
			"watch end requested.",
			"watch",
			watch.String())
	}

	r.watches = kept
}

//
// Transaction committed.
// Recorded (staged) events are forwarded to watches.
func (r *Journal) Report(staged *fb.List) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	for _, w := range r.watches {
		w.notify(staged.Iter())
	}
}

//
// Close the journal.
// End all watches.
func (r *Journal) Close() (err error) {
	for _, w := range r.watches {
		r.End(w)
	}

	r.log.V(3).Info("journal closed.")

	return
}

//
// Model is being watched.
// Determine if there a watch interested in the model.
func (r *Journal) hasWatch(model Model) bool {
	for _, w := range r.watches {
		if w.Match(model) {
			return true
		}
	}

	return false
}

//
// Stock event handler.
// Provides default event methods.
type StockEventHandler struct{}

// Watch options.
func (r *StockEventHandler) Options() WatchOptions {
	return WatchOptions{}
}

//
// Watch has started.
func (r *StockEventHandler) Started(uint64) {}

//
// Watch has parity.
func (r *StockEventHandler) Parity() {}

//
// A model has been created.
func (r *StockEventHandler) Created(Event) {}

//
// A model has been updated.
func (r *StockEventHandler) Updated(Event) {}

//
// A model has been deleted.
func (r *StockEventHandler) Deleted(Event) {}

//
// An error has occurred delivering an event.
func (r *StockEventHandler) Error(error) {}

//
// An event watch has ended.
func (r *StockEventHandler) End() {}

//
// Serial number pool.
type Serial struct {
	mutex sync.Mutex
	pool  map[int]uint64
}

//
// Next serial number.
func (r *Serial) next(key int) (sn uint64) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if r.pool == nil {
		r.pool = make(map[int]uint64)
	}
	sn = r.pool[key]
	sn++
	r.pool[key] = sn
	return
}
