package handler

import (
	"sync"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/provider/web"
	libweb "github.com/kubev2v/forklift/pkg/lib/inventory/web"
	libref "github.com/kubev2v/forklift/pkg/lib/ref"
	"k8s.io/apimachinery/pkg/types"
)

// Watch map keyed by resource kind.
type watchMap map[string]*libweb.Watch

// Provider map keyed by provider.UID.
type ProviderMap map[types.UID]watchMap

// Watch manager.
type WatchManager struct {
	mutex sync.Mutex
	// Provider map keyed by provider.UID.
	providerMap ProviderMap
}

// Ensure watch has been created.
// An existing watch that is not `alive` will be replaced.
func (m *WatchManager) Ensure(
	provider *api.Provider,
	resource interface{},
	handler libweb.EventHandler) (watch *libweb.Watch, err error) {
	//
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.providerMap == nil {
		m.providerMap = make(ProviderMap)
	}
	watchMap, found := m.providerMap[provider.UID]
	if !found {
		watchMap = make(map[string]*libweb.Watch)
		m.providerMap[provider.UID] = watchMap
	}
	kind := libref.ToKind(resource)
	if w, found := watchMap[kind]; found {
		if w.Alive() {
			watch = w
			return
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
	watchMap[kind] = w
	watch = w

	return
}

// A provider has been deleted.
// Delete associated watches.
func (m *WatchManager) Deleted(provider *api.Provider) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if watchMap, found := m.providerMap[provider.UID]; found {
		for _, w := range watchMap {
			w.End()
		}
	}

	delete(m.providerMap, provider.UID)
}
