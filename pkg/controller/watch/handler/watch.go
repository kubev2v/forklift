package handler

import (
	libweb "github.com/konveyor/controller/pkg/inventory/web"
	libref "github.com/konveyor/controller/pkg/ref"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web"
	"k8s.io/apimachinery/pkg/types"
	"sync"
)

//
// Watch map keyed by resource kind.
type watchMap map[string]*libweb.Watch

//
// Provider map keyed by provider.UID.
type ProviderMap map[types.UID]watchMap

//
// Watch manager.
type WatchManager struct {
	mutex sync.Mutex
	// Provider map keyed by provider.UID.
	providerMap ProviderMap
}

//
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
