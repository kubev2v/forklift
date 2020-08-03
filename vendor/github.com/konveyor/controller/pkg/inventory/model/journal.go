package model

import (
	liberr "github.com/konveyor/controller/pkg/error"
	"github.com/konveyor/controller/pkg/ref"
	"reflect"
	"sync"
)

//
// Event Actions.
var (
	Created int8 = 0x01
	Updated int8 = 0x02
	Deleted int8 = 0x04
)

//
// Model event.
type Event struct {
	// The event subject.
	Model Model
	// The event action (created|updated|deleted).
	Action int8
}

//
// Event handler.
type EventHandler interface {
	// A model has been created.
	Created(Model)
	// A model has been updated.
	Updated(Model)
	// A model has been deleted.
	Deleted(Model)
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
	// Event queue.
	queue chan *Event
	// Started
	started bool
}

//
// Match by model `kind`.
func (w *Watch) Match(model Model) bool {
	return ref.ToKind(w.Model) == ref.ToKind(model)
}

//
// Queue event.
func (w *Watch) notify(event *Event) {
	if !w.Match(event.Model) {
		return
	}
	defer func() {
		recover()
	}()
	select {
	case w.queue <- event:
	default:
		err := liberr.New("full queue, event discarded")
		w.Handler.Error(err)
	}
}

//
// Run the watch.
// Forward events to the `handler`.
func (w *Watch) Start() {
	if w.started {
		return
	}
	run := func() {
		for event := range w.queue {
			switch event.Action {
			case Created:
				w.Handler.Created(event.Model)
			case Updated:
				w.Handler.Updated(event.Model)
			case Deleted:
				w.Handler.Deleted(event.Model)
			default:
				w.Handler.Error(liberr.New("unknown action"))
			}
		}
		w.Handler.End()
	}

	w.started = true
	go run()
}

//
// End the watch.
func (w *Watch) End() {
	close(w.queue)
}

//
// Event manager.
type Journal struct {
	*Client
	mutex sync.RWMutex
	// List of registered watches.
	watches []*Watch
	// Queue of staged events.
	staged []*Event
	// Enabled.
	enabled bool
}

//
// The journal is enabled.
// Must be enabled for watch models.
func (r *Journal) Enabled() bool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.enabled
}

//
// Enable the journal.
func (r *Journal) Enable() {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.enabled = true
}

//
// Disable the journal.
// End all watches and discard staged events.
func (r *Journal) Disable() {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	for _, w := range r.watches {
		w.End()
	}
	r.watches = []*Watch{}
	r.staged = []*Event{}
	r.enabled = false
}

//
// Watch a `watch` of the specified model.
// The returned watch has not been started.
// See: Watch.Start().
func (r *Journal) Watch(model Model, handler EventHandler) (*Watch, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if !r.enabled {
		return nil, liberr.New("disabled")
	}
	watch := &Watch{
		Handler: handler,
		Model:   model,
	}
	r.watches = append(r.watches, watch)
	watch.queue = make(chan *Event, 10000)
	return watch, nil
}

//
// End watch.
func (r *Journal) End(watch *Watch) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if !r.enabled {
		return
	}
	kept := []*Watch{}
	for _, w := range r.watches {
		if w != watch {
			kept = append(kept, w)
			continue
		}
		w.End()
	}

	r.watches = kept
}

//
// A model has been created.
// Queue an event.
func (r *Journal) Created(model Model) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if !r.enabled {
		return
	}
	r.staged = append(
		r.staged,
		&Event{
			Model:  r.snapshot(model),
			Action: Created,
		})
}

//
// A model has been updated.
// Queue an event.
func (r *Journal) Updated(model Model) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if !r.enabled {
		return
	}
	r.staged = append(
		r.staged,
		&Event{
			Model:  r.snapshot(model),
			Action: Updated,
		})
}

//
// A model has been deleted.
// Queue an event.
func (r *Journal) Deleted(model Model) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if !r.enabled {
		return
	}
	r.staged = append(
		r.staged,
		&Event{
			Model:  r.snapshot(model),
			Action: Deleted,
		})
}

//
// Commit staged events and notify handlers.
func (r *Journal) Commit() {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if !r.enabled {
		return
	}
	for _, event := range r.staged {
		for _, w := range r.watches {
			w.notify(event)
		}
	}

	r.staged = []*Event{}
}

//
// Discard staged events.
func (r *Journal) Unstage() {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if !r.enabled {
		return
	}

	r.staged = []*Event{}
}

//
// Create a snapshot of the model.
// The model is a pointer must be protected against being
// changed at it origin or by the handlers.
func (r *Journal) snapshot(model Model) Model {
	mt := reflect.TypeOf(model)
	mv := reflect.ValueOf(model)
	switch mt.Kind() {
	case reflect.Ptr:
		mt = mt.Elem()
		mv = mv.Elem()
	}
	new := reflect.New(mt).Elem()
	new.Set(mv)
	return new.Addr().Interface().(Model)
}
