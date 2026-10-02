package web

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sync"

	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
)

// CacheStats tracks inventory cache effectiveness for a reconcile.
type CacheStats struct {
	FindHits   int
	FindMisses int
	VMHits     int
	VMMisses   int
	ListHits   int
	ListMisses int
}

// CachingClient wraps an inventory client and memoizes Find, VM, and List calls
// for the lifetime of the wrapper (typically one plan reconcile or migration begin).
type CachingClient struct {
	inner Client
	mu    sync.RWMutex
	find  map[string][]byte
	vm    map[string]vmCacheEntry
	list  map[string][]byte
	stats CacheStats
}

type vmCacheEntry struct {
	data []byte
	id   string
	name string
	typ  reflect.Type
}

// NewCachingClient wraps inner with a per-reconcile inventory cache.
func NewCachingClient(inner Client) Client {
	if inner == nil {
		return nil
	}
	if cached, ok := inner.(*CachingClient); ok {
		return cached
	}
	return &CachingClient{
		inner: inner,
		find:  map[string][]byte{},
		vm:    map[string]vmCacheEntry{},
		list:  map[string][]byte{},
	}
}

// Stats returns a snapshot of cache hit/miss counters.
func (r *CachingClient) Stats() CacheStats {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.stats
}

func (r *CachingClient) Finder() base.Finder {
	return r.inner.Finder()
}

func (r *CachingClient) Get(resource interface{}, id string) error {
	return r.inner.Get(resource, id)
}

func (r *CachingClient) Watch(resource interface{}, h EventHandler) (*Watch, error) {
	return r.inner.Watch(resource, h)
}

func (r *CachingClient) Find(resource interface{}, ref base.Ref) error {
	key := findCacheKey(resource, ref)
	r.mu.RLock()
	data, ok := r.find[key]
	r.mu.RUnlock()
	if ok {
		r.mu.Lock()
		r.stats.FindHits++
		r.mu.Unlock()
		return json.Unmarshal(data, resource)
	}

	err := r.inner.Find(resource, ref)
	if err != nil {
		return err
	}
	data, err = json.Marshal(resource)
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.find[key] = data
	r.stats.FindMisses++
	r.mu.Unlock()
	return nil
}

func (r *CachingClient) VM(ref *base.Ref) (object interface{}, err error) {
	key := refCacheKey(*ref)
	r.mu.RLock()
	entry, ok := r.vm[key]
	r.mu.RUnlock()
	if ok {
		r.mu.Lock()
		r.stats.VMHits++
		r.mu.Unlock()
		ref.ID = entry.id
		ref.Name = entry.name
		return unmarshalVM(entry.data, entry.typ)
	}

	object, err = r.inner.VM(ref)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(object)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	r.vm[key] = vmCacheEntry{data: data, id: ref.ID, name: ref.Name, typ: reflect.TypeOf(object)}
	r.stats.VMMisses++
	r.mu.Unlock()
	return unmarshalVM(data, reflect.TypeOf(object))
}

func (r *CachingClient) Workload(ref *base.Ref) (object interface{}, err error) {
	return r.inner.Workload(ref)
}

func (r *CachingClient) Network(ref *base.Ref) (object interface{}, err error) {
	return r.inner.Network(ref)
}

func (r *CachingClient) Storage(ref *base.Ref) (object interface{}, err error) {
	return r.inner.Storage(ref)
}

func (r *CachingClient) Host(ref *base.Ref) (object interface{}, err error) {
	return r.inner.Host(ref)
}

func (r *CachingClient) List(resource interface{}, param ...base.Param) error {
	key := listCacheKey(resource, param)
	r.mu.RLock()
	data, ok := r.list[key]
	r.mu.RUnlock()
	if ok {
		r.mu.Lock()
		r.stats.ListHits++
		r.mu.Unlock()
		return json.Unmarshal(data, resource)
	}

	err := r.inner.List(resource, param...)
	if err != nil {
		return err
	}
	data, err = json.Marshal(resource)
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.list[key] = data
	r.stats.ListMisses++
	r.mu.Unlock()
	return nil
}

func findCacheKey(resource interface{}, ref base.Ref) string {
	return fmt.Sprintf("%s|%s", reflect.TypeOf(resource).String(), refCacheKey(ref))
}

func refCacheKey(ref base.Ref) string {
	if ref.ID != "" {
		return ref.ID
	}
	return ref.Name
}

func listCacheKey(resource interface{}, param []base.Param) string {
	key := reflect.TypeOf(resource).String()
	for _, p := range param {
		key += fmt.Sprintf("|%s=%s", p.Key, p.Value)
	}
	return key
}

func unmarshalVM(data []byte, typ reflect.Type) (interface{}, error) {
	if typ == nil {
		return nil, fmt.Errorf("missing VM type in cache")
	}
	out := reflect.New(typ.Elem()).Interface()
	if err := json.Unmarshal(data, out); err != nil {
		return nil, err
	}
	return out, nil
}
