package collector

import (
	"context"
	"errors"
	"fmt"
	"path"
	"sync"
	"time"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	libcontainer "github.com/kubev2v/forklift/pkg/lib/inventory/container"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	ec2client "github.com/kubev2v/forklift/pkg/provider/ec2/inventory/client"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Collector periodically fetches EC2 inventory from AWS and caches it in local database.
// Polls AWS APIs at intervals, transforms responses to model format, updates DB for fast querying.
// Provides performance optimization layer for migration planning, validation, and UI operations.
type Collector struct {
	libcontainer.Collector
	db         libmodel.DB         // Local inventory database
	provider   *api.Provider       // Provider CR configuration
	secret     *core.Secret        // AWS credentials
	client     *ec2client.Client   // AWS SDK client
	log        logging.LevelLogger // Structured logger
	cancel     context.CancelFunc  // Stop collection loop
	parity     bool                // True when inventory synchronized with AWS
	collecting bool                // True when collection in progress
	mutex      sync.Mutex          // Protects 'collecting' flag
}

// New creates a new EC2 inventory collector with database, provider CR, and AWS credentials.
// Collector is initialized but not started - call Start() to begin inventory collection.
func New(db libmodel.DB, provider *api.Provider, secret *core.Secret) libcontainer.Collector {
	log := logging.WithName("collector|ec2").WithValues(
		"provider",
		path.Join(
			provider.GetNamespace(),
			provider.GetName()))

	return &Collector{
		db:       db,
		provider: provider,
		secret:   secret,
		log:      log,
	}
}

// Name returns the identifier for this collector type.
func (r *Collector) Name() string {
	return "EC2"
}

// HasParity returns whether the inventory database is synchronized with the AWS account.
func (r *Collector) HasParity() bool {
	return r.parity
}

// Start initializes AWS client, performs initial inventory collection, then begins periodic refresh loop.
// Runs collection tasks (instances, volumes, networks) at RefreshInterval. Uses mutex to prevent
// concurrent collections. Continues running until Shutdown() called. Sets parity=true after successful collection.
func (r *Collector) Start() error {
	client, err := ec2client.New(r.provider, r.secret)
	if err != nil {
		return err
	}
	r.client = client

	ctx := context.Background()
	ctx, r.cancel = context.WithCancel(ctx)

	start := func() {
		defer func() {
			r.log.Info("Collection loop stopped.")
		}()

		if err := r.Collect(); err != nil {
			r.log.Error(err, "Initial collection failed")
		} else {
			r.parity = true
			r.log.Info("Initial collection completed, parity achieved.")
		}

		ticker := time.NewTicker(RefreshInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.log.V(1).Info("Starting periodic collection")
				if err := r.Collect(); err != nil {
					r.log.Error(err, "Periodic collection failed")
					r.parity = false
				} else {
					r.parity = true
					r.log.V(1).Info("Periodic collection completed")
				}
			}
		}
	}

	go start()

	r.log.Info("Collector started.")
	return nil
}

// Shutdown gracefully stops the collector's periodic collection loop.
func (r *Collector) Shutdown() {
	if r.cancel != nil {
		r.cancel()
	}
	r.log.Info("Collector shut down.")
}

// Reset marks the inventory as out of sync, forcing a full refresh on next collection.
func (r *Collector) Reset() {
	r.parity = false
	r.log.Info("Collector reset.")
}

// collectionTask represents inventory collection for one resource type (instances, volumes, networks).
// Independent tasks allow partial success, error isolation, progress tracking per resource type.
// Each calls AWS API, transforms to models, updates DB, handles errors.
type collectionTask struct {
	name string // Task name for logging (e.g., "instances")

	// fn is the function that performs the actual collection for this resource type.
	// It receives a context for cancellation and returns an error if collection fails.
	// Example functions: collectInstances, collectVolumes, collectNetworks
	fn func(context.Context) error
}

// Collect performs full inventory collection from AWS EC2 APIs.
// Runs independent tasks for instances, volumes, and networks in sequence. Uses mutex to prevent
// concurrent collections. Updates database with discovered resources. Returns first error encountered
// but continues collecting other resource types for partial success.
func (r *Collector) Collect() error {
	r.mutex.Lock()
	if r.collecting {
		r.mutex.Unlock()
		r.log.Info("Collection already in progress, skipping")
		return nil
	}
	r.collecting = true
	r.mutex.Unlock()

	defer func() {
		r.mutex.Lock()
		r.collecting = false
		r.mutex.Unlock()
	}()

	ctx := context.TODO()
	r.log.V(1).Info("Starting collection")

	// Define collection tasks
	tasks := []collectionTask{
		{"instances", r.collectInstances},
		{"volumes", r.collectVolumes},
		{"networks", r.collectNetworks},
		{"storageTypes", r.collectStorageTypes},
	}

	// Execute all collection tasks
	var errs []error
	successCount := 0

	for _, task := range tasks {
		if err := task.fn(ctx); err != nil {
			r.log.Error(err, "Failed to collect "+task.name)
			errs = append(errs, fmt.Errorf("%s: %w", task.name, err))
		} else {
			successCount++
		}
	}

	// Handle results
	if len(errs) == 0 {
		r.log.V(1).Info("Collection completed successfully", "total", len(tasks))
		return nil
	}

	if len(errs) == len(tasks) {
		r.log.Error(nil, "All collections failed")
		return errors.Join(errs...)
	}

	// Partial success - log but don't fail
	r.log.Info("Collection partially completed",
		"successful", successCount,
		"failed", len(errs),
		"total", len(tasks))
	r.log.V(2).Info("Partial collection errors", "errors", errors.Join(errs...))
	return nil
}

// DB returns the database
func (r *Collector) DB() libmodel.DB {
	return r.db
}

// Version returns version information (NO-OP for EC2)
func (r *Collector) Version() (version, product, apiVersion, instanceUuid string, err error) {
	version = "ec2"
	product = "AWS EC2"
	return
}

// Owner returns the owner
func (r *Collector) Owner() meta.Object {
	return r.provider
}

// Test tests the connection to AWS EC2
func (r *Collector) Test() (status int, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Initialize client if not already initialized (needed for webhook validation)
	if r.client == nil {
		r.client, err = ec2client.New(r.provider, r.secret)
		if err != nil {
			// Return 401 for credential errors, 400 for other errors (e.g., missing region)
			status = 400
			return
		}
	}

	// Test connection by attempting to describe instances
	_, err = r.client.DescribeInstances(ctx)
	if err != nil {
		status = 500
		return
	}

	status = 200
	return
}
