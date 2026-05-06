package persistent

import (
	"context"
	"sync"
	"time"

	"github.com/kubev2v/vm-migration-detective/internal/inspection"
	"github.com/kubev2v/vm-migration-detective/internal/vddk"
	"github.com/kubev2v/vm-migration-detective/pkg/types"
	"github.com/sirupsen/logrus"
)

// Use types from pkg/types
type Credentials = types.Credentials
type CacheKey = types.CacheKey
type DB = types.DB

// InspectorInterface defines the interface for VM inspection operations
type InspectorInterface interface {
	// InspectWithVirt performs inspection using VirtInspector with memory and DB caching
	InspectWithVirt(ctx context.Context, vmMoref string, snapshotMoref string, diskInfo *types.SnapshotDiskInfo) (*types.VirtInspectorXML, error)

	// InspectWithVirtV2v performs inspection using VirtV2vInspector with memory and DB caching
	InspectWithVirtV2v(ctx context.Context, vmMoref string, snapshotMoref string, diskInfo *types.SnapshotDiskInfo, sslVerify string) (*types.VirtV2VInspectorXML, error)

	// GetDB returns the database instance used by the inspector
	GetDB() DB
}

// Inspector wraps both VirtInspector and VirtV2vInspector with memory and DB persistence
type Inspector struct {
	virtInspector      *inspection.VirtInspector
	virtV2vInspector   *inspection.VirtV2vInspector
	db                 DB
	credentials        Credentials
	virtMemoryCache    *virtInspectorMemoryCache
	virtV2vMemoryCache *virtV2vInspectorMemoryCache
	virtInflight       *inflightTracker[*types.VirtInspectorXML]
	virtV2vInflight    *inflightTracker[*types.VirtV2VInspectorXML]
	logger             *logrus.Logger
}

// NewInspector creates a new Inspector that supports both inspection methods
// virtInspectorPath: path to virt-inspector executable (uses system PATH if empty)
// virtV2vInspectorPath: path to virt-v2v-inspector executable (uses system PATH if empty)
// timeout: timeout for inspection operations (defaults to 5 minutes if zero)
// credentials: vCenter access credentials
// logger: logger instance for logging (can be nil)
// db: database implementation provided by caller (can be nil for memory-only caching)
// vddkLibDir: path to VDDK library directory (required, cannot be empty)
func NewInspector(virtInspectorPath string, virtV2vInspectorPath string, timeout time.Duration, credentials Credentials, logger *logrus.Logger, db DB, vddkLibDir string) *Inspector {
	// Set VDDK library directory for internal use
	// Caller must provide vddkLibDir - no fallback to default locations
	vddk.SetLibDir(vddkLibDir)

	return &Inspector{
		virtInspector:      inspection.NewVirtInspector(virtInspectorPath, timeout, logger),
		virtV2vInspector:   inspection.NewVirtV2vInspector(virtV2vInspectorPath, timeout, logger),
		db:                 db,
		credentials:        credentials,
		virtMemoryCache:    newVirtInspectorMemoryCache(),
		virtV2vMemoryCache: newVirtV2vInspectorMemoryCache(),
		virtInflight:       newInflightTracker[*types.VirtInspectorXML](),
		virtV2vInflight:    newInflightTracker[*types.VirtV2VInspectorXML](),
		logger:             logger,
	}
}

// InspectWithVirt performs inspection using VirtInspector with memory and DB caching
// Concurrent calls for the same VM-snapshot key will wait for the first call to complete
func (p *Inspector) InspectWithVirt(
	ctx context.Context,
	vmMoref string,
	snapshotMoref string,
	diskInfo *types.SnapshotDiskInfo,
) (*types.VirtInspectorXML, error) {
	key := CacheKey{
		VMMoref:       vmMoref,
		SnapshotMoref: snapshotMoref,
	}

	// Check memory cache first
	if cached := p.virtMemoryCache.get(key); cached != nil {
		if p.logger != nil {
			p.logger.WithFields(logrus.Fields{
				"vm_moref":       vmMoref,
				"snapshot_moref": snapshotMoref,
			}).Debug("Inspection data found in memory cache")
		}
		return cached, nil
	}

	// Check if there's already an inflight request for this key
	// If yes, wait for it; if no, we become the one doing the work
	result, err, isWaiter := p.virtInflight.do(key, func() (*types.VirtInspectorXML, error) {
		// Double-check memory cache (another goroutine might have populated it)
		if cached := p.virtMemoryCache.get(key); cached != nil {
			if p.logger != nil {
				p.logger.WithFields(logrus.Fields{
					"vm_moref":       vmMoref,
					"snapshot_moref": snapshotMoref,
				}).Debug("Inspection data found in memory cache (double-check)")
			}
			return cached, nil
		}

		// Check DB if provided
		if p.db != nil {
			cached, err := p.db.GetVirtInspectorXML(ctx, key)
			if err != nil {
				if p.logger != nil {
					p.logger.WithError(err).Warn("Failed to get inspection data from DB")
				}
			} else if cached != nil {
				if p.logger != nil {
					p.logger.WithFields(logrus.Fields{
						"vm_moref":       vmMoref,
						"snapshot_moref": snapshotMoref,
					}).Debug("Inspection data found in DB")
				}
				// Store in memory cache for faster subsequent access
				p.virtMemoryCache.set(key, cached)
				return cached, nil
			}
		}

		// Perform actual inspection
		if p.logger != nil {
			p.logger.WithFields(logrus.Fields{
				"vm_moref":       vmMoref,
				"snapshot_moref": snapshotMoref,
			}).Info("Performing new inspection (not found in cache)")
		}

		result, err := p.virtInspector.Inspect(ctx, vmMoref, snapshotMoref, p.credentials.VCenterURL, p.credentials.Username, p.credentials.Password, diskInfo)
		if err != nil {
			return nil, err
		}

		// Store in memory cache
		p.virtMemoryCache.set(key, result)

		// Store in DB if provided
		if p.db != nil {
			if err := p.db.SetVirtInspectorXML(ctx, key, result); err != nil {
				if p.logger != nil {
					p.logger.WithError(err).Warn("Failed to store inspection data in DB")
				}
				// Don't fail the inspection if DB storage fails
			}
		}

		return result, nil
	})

	if isWaiter && p.logger != nil {
		p.logger.WithFields(logrus.Fields{
			"vm_moref":       vmMoref,
			"snapshot_moref": snapshotMoref,
		}).Debug("Waited for inflight inspection to complete")
	}

	return result, err
}

// InspectWithVirtV2v performs inspection using VirtV2vInspector with memory and DB caching
// Concurrent calls for the same VM-snapshot key will wait for the first call to complete
func (p *Inspector) InspectWithVirtV2v(
	ctx context.Context,
	vmMoref string,
	snapshotMoref string,
	diskInfo *types.SnapshotDiskInfo,
	sslVerify string,
) (*types.VirtV2VInspectorXML, error) {
	key := CacheKey{
		VMMoref:       vmMoref,
		SnapshotMoref: snapshotMoref,
	}

	// Check memory cache first
	if cached := p.virtV2vMemoryCache.get(key); cached != nil {
		if p.logger != nil {
			p.logger.WithFields(logrus.Fields{
				"vm_moref":       vmMoref,
				"snapshot_moref": snapshotMoref,
			}).Debug("Inspection data found in memory cache")
		}
		return cached, nil
	}

	// Check if there's already an inflight request for this key
	// If yes, wait for it; if no, we become the one doing the work
	result, err, isWaiter := p.virtV2vInflight.do(key, func() (*types.VirtV2VInspectorXML, error) {
		// Double-check memory cache (another goroutine might have populated it)
		if cached := p.virtV2vMemoryCache.get(key); cached != nil {
			if p.logger != nil {
				p.logger.WithFields(logrus.Fields{
					"vm_moref":       vmMoref,
					"snapshot_moref": snapshotMoref,
				}).Debug("Inspection data found in memory cache (double-check)")
			}
			return cached, nil
		}

		// Check DB if provided
		if p.db != nil {
			cached, err := p.db.GetVirtV2VInspectorXML(ctx, key)
			if err != nil {
				if p.logger != nil {
					p.logger.WithError(err).Warn("Failed to get inspection data from DB")
				}
			} else if cached != nil {
				if p.logger != nil {
					p.logger.WithFields(logrus.Fields{
						"vm_moref":       vmMoref,
						"snapshot_moref": snapshotMoref,
					}).Debug("Inspection data found in DB")
				}
				// Store in memory cache for faster subsequent access
				p.virtV2vMemoryCache.set(key, cached)
				return cached, nil
			}
		}

		// Perform actual inspection
		if p.logger != nil {
			p.logger.WithFields(logrus.Fields{
				"vm_moref":       vmMoref,
				"snapshot_moref": snapshotMoref,
			}).Info("Performing new inspection (not found in cache)")
		}

		result, err := p.virtV2vInspector.Inspect(ctx, vmMoref, snapshotMoref, p.credentials.VCenterURL, p.credentials.Username, p.credentials.Password, diskInfo, sslVerify)
		if err != nil {
			return nil, err
		}

		// Store in memory cache
		p.virtV2vMemoryCache.set(key, result)

		// Store in DB if provided
		if p.db != nil {
			if err := p.db.SetVirtV2VInspectorXML(ctx, key, result); err != nil {
				if p.logger != nil {
					p.logger.WithError(err).Warn("Failed to store inspection data in DB")
				}
				// Don't fail the inspection if DB storage fails
			}
		}

		return result, nil
	})

	if isWaiter && p.logger != nil {
		p.logger.WithFields(logrus.Fields{
			"vm_moref":       vmMoref,
			"snapshot_moref": snapshotMoref,
		}).Debug("Waited for inflight inspection to complete")
	}

	return result, err
}

// virtInspectorMemoryCache provides in-memory caching for VirtInspector results
type virtInspectorMemoryCache struct {
	mu    sync.RWMutex
	cache map[string]*types.VirtInspectorXML
}

// newVirtInspectorMemoryCache creates a new in-memory cache
func newVirtInspectorMemoryCache() *virtInspectorMemoryCache {
	return &virtInspectorMemoryCache{
		cache: make(map[string]*types.VirtInspectorXML),
	}
}

// get retrieves data from memory cache
func (c *virtInspectorMemoryCache) get(key CacheKey) *types.VirtInspectorXML {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cache[key.String()]
}

// set stores data in memory cache
func (c *virtInspectorMemoryCache) set(key CacheKey, data *types.VirtInspectorXML) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[key.String()] = data
}

// virtV2vInspectorMemoryCache provides in-memory caching for VirtV2vInspector results
type virtV2vInspectorMemoryCache struct {
	mu    sync.RWMutex
	cache map[string]*types.VirtV2VInspectorXML
}

// newVirtV2vInspectorMemoryCache creates a new in-memory cache
func newVirtV2vInspectorMemoryCache() *virtV2vInspectorMemoryCache {
	return &virtV2vInspectorMemoryCache{
		cache: make(map[string]*types.VirtV2VInspectorXML),
	}
}

// get retrieves data from memory cache
func (c *virtV2vInspectorMemoryCache) get(key CacheKey) *types.VirtV2VInspectorXML {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cache[key.String()]
}

// set stores data in memory cache
func (c *virtV2vInspectorMemoryCache) set(key CacheKey, data *types.VirtV2VInspectorXML) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[key.String()] = data
}

// inflightCall represents an ongoing inspection call
type inflightCall[T any] struct {
	wg  sync.WaitGroup
	val T
	err error
}

// inflightTracker tracks ongoing inspection requests per key
// Ensures only one inspection runs per key, with concurrent requests waiting
type inflightTracker[T any] struct {
	mu    sync.Mutex
	calls map[string]*inflightCall[T]
}

// newInflightTracker creates a new inflight tracker
func newInflightTracker[T any]() *inflightTracker[T] {
	return &inflightTracker[T]{
		calls: make(map[string]*inflightCall[T]),
	}
}

// do executes the given function for the key, or waits if another goroutine is already executing it
// Returns: (result, error, isWaiter)
// - isWaiter is true if this call waited for another goroutine's result
// - isWaiter is false if this call actually executed the function
func (t *inflightTracker[T]) do(key CacheKey, fn func() (T, error)) (T, error, bool) {
	keyStr := key.String()

	t.mu.Lock()
	if call, exists := t.calls[keyStr]; exists {
		// Another goroutine is already working on this key
		t.mu.Unlock()

		// Wait for it to complete
		call.wg.Wait()

		return call.val, call.err, true
	}

	// We are the first one for this key - create a new call
	call := &inflightCall[T]{}
	call.wg.Add(1)
	t.calls[keyStr] = call
	t.mu.Unlock()

	// Execute the function
	call.val, call.err = fn()

	// Mark as done
	call.wg.Done()

	// Remove from inflight map
	t.mu.Lock()
	delete(t.calls, keyStr)
	t.mu.Unlock()

	return call.val, call.err, false
}

// GetDB returns the database instance used by the inspector
func (p *Inspector) GetDB() DB {
	return p.db
}
