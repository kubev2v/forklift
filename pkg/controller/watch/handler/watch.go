package handler

import (
	"path"
	"sync"
	"time"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/provider/web"
	libweb "github.com/kubev2v/forklift/pkg/lib/inventory/web"
	libref "github.com/kubev2v/forklift/pkg/lib/ref"
	"k8s.io/apimachinery/pkg/types"
)

const (
	DefaultEventInterval = time.Second * 10
)

// A stoppable resource.
type Stoppable interface {
	// End the watch.
	End()
}

// Watch map keyed by resource kind.
type watchMap map[string]Stoppable

// Provider map keyed by provider.UID.
type ProviderMap map[types.UID]watchMap

// Watch manager.
type WatchManager struct {
	mutex sync.Mutex
	// Provider map keyed by provider.UID.
	providerMap ProviderMap
}

func (m *WatchManager) ensureStoppablesUnlocked(
	provider *api.Provider) *watchMap {
	if m.providerMap == nil {
		m.providerMap = make(ProviderMap)
	}
	stoppables, found := m.providerMap[provider.UID]
	if !found {
		stoppables = make(map[string]Stoppable)
		m.providerMap[provider.UID] = stoppables
	}

	return &stoppables
}

// Ensure a watch has been created.
func (m *WatchManager) Ensure(
	provider *api.Provider,
	resource interface{},
	handler libweb.EventHandler) (watch *libweb.Watch, err error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	stoppables := m.ensureStoppablesUnlocked(provider)
	kind := libref.ToKind(resource)
	if s, found := (*stoppables)[kind]; found {
		if w, cast := s.(*libweb.Watch); cast {
			if w.Alive() {
				watch = w
				return
			}
		} else {
			log.Info("Creating a new watch for a resource that already has a periodic update")
			delete(*stoppables, kind)
		}
	}

	client, err := web.NewClient(provider)
	if err != nil {
		return
	}
	w, err := client.Watch(resource, handler)
	if err != nil {
		return
	}
	(*stoppables)[kind] = w
	watch = w

	return
}

type PeriodicEventGenerator struct {
	stopChannel chan struct{}
}

func (r *PeriodicEventGenerator) End() {
	close(r.stopChannel)
}

// Ensure that we've started a periodic event generator for the provider
// resource.
func (m *WatchManager) EnsurePeriodicEvents(
	provider *api.Provider,
	resource interface{},
	interval time.Duration,
	tickFunc func()) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	stoppables := m.ensureStoppablesUnlocked(provider)
	kind := libref.ToKind(resource)
	if _, found := (*stoppables)[kind]; found {
		return
	}
	eventGenerator := &PeriodicEventGenerator{stopChannel: make(chan struct{})}
	log.Info(
		"Periodic event generator started.",
		"provider",
		path.Join(provider.Namespace, provider.Name))

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				tickFunc()
			case <-eventGenerator.stopChannel:
				log.Info("Periodic event generator stopped.",
					"provider",
					path.Join(provider.Namespace, provider.Name))
				return
			}
		}
	}()

	(*stoppables)[kind] = eventGenerator
}

// A provider has been deleted.
func (m *WatchManager) Deleted(provider *api.Provider) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if stoppables, found := m.providerMap[provider.UID]; found {
		for _, s := range stoppables {
			s.End()
		}
		delete(m.providerMap, provider.UID)
		log.Info(
			"Watch stopped.",
			"provider",
			path.Join(
				provider.Namespace,
				provider.Name))
	}
}

// End all watches.
func (m *WatchManager) End() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	for _, stoppables := range m.providerMap {
		for _, s := range stoppables {
			s.End()
		}
	}
	clear(m.providerMap)
}
