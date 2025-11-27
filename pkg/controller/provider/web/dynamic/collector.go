package dynamic

import (
	"context"
	"sync"
	"time"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Settings
const (
	// Retry interval.
	RetryInterval = 5 * time.Second
	// Default refresh interval - poll inventory from dynamic provider.
	// This can be overridden by DynamicProviderServer.Spec.RefreshInterval.
	// Set to 5 minutes to accommodate large inventories that may take
	// several minutes to scan completely.
	DefaultRefreshInterval = 5 * time.Minute
)

// Phases
const (
	Started = ""
	Load    = "load"
	Loaded  = "loaded"
	Parity  = "parity"
	Refresh = "refresh"
)

// Collector for dynamic providers with polling support
// Polls external service periodically to mimic event-based updates
type Collector struct {
	provider *api.Provider
	config   *ProviderConfig
	// DB client for caching inventory
	db libmodel.DB
	// has parity
	parity bool
	// cancel function
	cancel context.CancelFunc
	// phase
	phase string
	// List of watches
	watches []*libmodel.Watch
	// refresh interval (from DynamicProviderServer spec)
	refreshInterval time.Duration
	// forceRefresh channel for immediate refresh trigger
	forceRefresh chan struct{}
	// refreshMutex prevents concurrent refresh operations
	refreshMutex sync.Mutex
	// shutdownOnce ensures shutdown is only called once
	shutdownOnce sync.Once
	// wg waits for collector goroutine to finish
	wg sync.WaitGroup
}

func New(provider *api.Provider, db libmodel.DB) *Collector {
	config, _ := Registry.Get(string(provider.Type()))

	// Get refresh interval from provider config, default to 5 minutes
	refreshInterval := DefaultRefreshInterval
	if config != nil && config.RefreshInterval > 0 {
		refreshInterval = time.Duration(config.RefreshInterval) * time.Second
	}

	return &Collector{
		provider:        provider,
		config:          config,
		db:              db,
		phase:           Started,
		refreshInterval: refreshInterval,
		forceRefresh:    make(chan struct{}, 1), // Buffered to avoid blocking
	}
}

func (r *Collector) Name() string {
	return r.provider.Name
}

func (r *Collector) Owner() meta.Object {
	return r.provider
}

func (r *Collector) DB() libmodel.DB {
	return r.db
}

func (r *Collector) Reset() {
	r.parity = false
	r.phase = Started

	// Signal immediate refresh (non-blocking)
	select {
	case r.forceRefresh <- struct{}{}:
		log.V(3).Info("Force refresh signal sent",
			"provider", r.provider.Name)
	default:
		// Channel full, already has pending refresh
		log.V(3).Info("Force refresh already pending",
			"provider", r.provider.Name)
	}
}

func (r *Collector) HasParity() bool {
	return r.parity
}

func (r *Collector) Start() error {
	ctx := context.Background()
	ctx, r.cancel = context.WithCancel(ctx)

	r.wg.Add(1)
	start := func() {
		defer func() {
			r.endWatch()
			r.wg.Done()
			log.Info("Dynamic provider collector stopped",
				"provider", r.provider.Name)
		}()

		for {
			select {
			case <-ctx.Done():
				return
			default:
				_ = r.run(ctx)
			}
		}
	}

	go start()

	log.Info("Dynamic provider collector started",
		"provider", r.provider.Name,
		"type", r.provider.Type())

	return nil
}

// Run the current phase
func (r *Collector) run(ctx context.Context) (err error) {
	log.V(3).Info("Running phase",
		"provider", r.provider.Name,
		"phase", r.phase)

	switch r.phase {
	case Started:
		r.phase = Load
	case Load:
		err = r.load(ctx)
		if err == nil {
			r.phase = Loaded
		}
	case Loaded:
		// Try to acquire lock (non-blocking)
		if !r.refreshMutex.TryLock() {
			log.V(3).Info("Refresh already in progress, skipping",
				"provider", r.provider.Name)
			return nil
		}
		err = r.refresh(ctx)
		r.refreshMutex.Unlock()
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
		// Try to acquire lock (non-blocking)
		if !r.refreshMutex.TryLock() {
			log.V(3).Info("Refresh already in progress, skipping",
				"provider", r.provider.Name)
			return nil
		}
		err = r.refresh(ctx)
		r.refreshMutex.Unlock()
		if err == nil {
			r.parity = true
			// Sleep between refreshes (interruptible by force refresh or shutdown)
			sleepDuration := r.refreshInterval
			if sleepDuration == 0 {
				// Polling disabled - sleep longer to avoid busy loop
				sleepDuration = 5 * time.Minute
			}

			// Wait for next refresh (can be interrupted)
			select {
			case <-time.After(sleepDuration):
				// Normal refresh interval elapsed
			case <-r.forceRefresh:
				// Force refresh requested
				log.V(3).Info("Force refresh triggered, skipping sleep",
					"provider", r.provider.Name)
			case <-ctx.Done():
				// Shutdown requested
				return ctx.Err()
			}
		} else {
			r.parity = false
		}
	default:
		err = liberr.New("Phase unknown")
	}

	if err != nil {
		log.Error(err, "Dynamic provider collector failed",
			"provider", r.provider.Name,
			"phase", r.phase)
		time.Sleep(RetryInterval)
	}

	return
}

func (r *Collector) Shutdown() {
	r.shutdownOnce.Do(func() {
		log.Info("Shutting down dynamic provider collector",
			"provider", r.provider.Name)
		if r.cancel != nil {
			r.cancel()
		}
		if r.forceRefresh != nil {
			close(r.forceRefresh)
		}
		// Wait for the collector goroutine to finish
		r.wg.Wait()
		log.Info("Dynamic provider collector shutdown complete",
			"provider", r.provider.Name)
	})
}
