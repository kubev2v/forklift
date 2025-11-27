package main

import (
	"net/http/httputil"
	"sync"
	"time"
)

type CachedProxy struct {
	Proxy    *httputil.ReverseProxy
	CachedAt time.Time
}

type ProxyCache struct {
	cache map[string]CachedProxy
	mutex sync.Mutex
	TTL   time.Duration
}

func (r *ProxyCache) Add(key string, value *httputil.ReverseProxy) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if r.cache == nil {
		r.cache = make(map[string]CachedProxy)
	}
	r.cache[key] = CachedProxy{
		Proxy:    value,
		CachedAt: time.Now(),
	}
}

func (r *ProxyCache) Get(key string) (proxy *httputil.ReverseProxy, found bool) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	result, ok := r.cache[key]
	if ok {
		if time.Since(result.CachedAt) <= r.TTL {
			proxy = result.Proxy
			found = true
		} else {
			delete(r.cache, key)
		}
	}
	return
}

func NewProxyCache(ttl int64) (cache *ProxyCache) {
	cache = &ProxyCache{
		TTL:   time.Duration(ttl) * time.Second,
		cache: make(map[string]CachedProxy),
	}
	return
}
