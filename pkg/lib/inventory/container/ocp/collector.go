package ocp

import (
	"context"
	"fmt"
	"path"
	"time"

	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	ocp "github.com/konveyor/forklift-controller/pkg/lib/client/openshift"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	libmodel "github.com/konveyor/forklift-controller/pkg/lib/inventory/model"
	"github.com/konveyor/forklift-controller/pkg/lib/logging"
	"github.com/konveyor/forklift-controller/pkg/lib/ref"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	RetryDelay = time.Second * 5
)

// Cluster.
type Cluster interface {
	meta.Object
}

// An OpenShift collector.
type Collector struct {
	// The cluster CR.
	cluster Cluster
	// DB client.
	db libmodel.DB
	// Credentials secret.
	secret *core.Secret
	// Logger.
	log logging.LevelLogger
	// Collections
	collections []Collection
	// The k8s manager.
	manager manager.Manager
	// A k8s non-cached client.
	client client.Client
	// Model event channel.
	eventChannel chan ModelEvent
	// The model version threshold used to determine if a
	// model event is obsolete. An event (model) with a version
	// lower than the threshold is redundant to changes made
	// during collection reconciliation.
	versionThreshold uint64
	// The collector has (initial) parity.
	parity bool
	// cancel function.
	cancel func()
}

// New collector.
func New(
	db libmodel.DB,
	cluster Cluster,
	secret *core.Secret,
	collections ...Collection) *Collector {
	//
	log := logging.WithName("collector|ocp").WithValues(
		"cluster",
		path.Join(
			cluster.GetNamespace(),
			cluster.GetName()))
	return &Collector{
		collections: collections,
		cluster:     cluster,
		secret:      secret,
		log:         log,
		db:          db,
	}
}

// The name.
func (r *Collector) Name() string {
	return r.cluster.GetName()
}

// The owner.
func (r *Collector) Owner() meta.Object {
	return r.cluster
}

// Get the DB.
func (r *Collector) DB() libmodel.DB {
	return r.db
}

// Get the Client.
func (r *Collector) Client() client.Client {
	return r.client
}

// Reset.
func (r *Collector) Reset() {
	r.parity = false
}

// Collector has achieved parity.
func (r *Collector) HasParity() bool {
	return r.parity
}

// Follow link
func (r *Collector) Follow(moRef interface{}, p []string, dst interface{}) error {
	return fmt.Errorf("not implemented")
}

// Update the versionThreshold
func (r *Collector) UpdateThreshold(m libmodel.Model) {
	if m, cast := m.(interface{ ResourceVersion() uint64 }); cast {
		n := m.ResourceVersion()
		if n > r.versionThreshold {
			r.versionThreshold = n
		}
	}
}

// Test connection with credentials.
func (r *Collector) Test() (int, error) {
	return 0, r.buildClient()
}

// Start the collector.
func (r *Collector) Start() error {
	ctx := context.Background()
	ctx, r.cancel = context.WithCancel(ctx)
	for _, collection := range r.collections {
		collection.Bind(r)
	}
	start := func() {
	try:
		for {
			select {
			case <-ctx.Done():
				break try
			default:
				err := r.start(ctx)
				if err != nil {
					r.log.V(3).Error(
						err,
						"start failed.",
						"retry",
						RetryDelay)
					time.Sleep(RetryDelay)
					continue try
				}
				break try
			}
		}
	}

	go start()

	return nil
}

// Start details.
//  1. Build and start the manager.
//  2. Reconcile all of the collections.
//  3. Mark parity.
//  4. Start apply events (coroutine).
func (r *Collector) start(ctx context.Context) (err error) {
	r.versionThreshold = 0
	r.eventChannel = make(chan ModelEvent, 100)
	defer func() {
		if err != nil {
			r.terminate()
		}
	}()
	mark := time.Now()
	r.log.V(3).Info("starting.")
	err = r.buildManager()
	if err != nil {
		return
	}
	err = r.buildClient()
	if err != nil {
		return
	}
	go func() {
		if err := r.manager.Start(ctx); err != nil {
			r.log.V(3).Error(err, "manager failed.")
		}
	}()

	err = r.reconcileCollections(ctx)
	if err != nil {
		return
	}
	go r.applyEvents()

	r.log.V(3).Info(
		"started.",
		"duration",
		time.Since(mark))

	return
}

// Reconcile collections.
func (r *Collector) reconcileCollections(ctx context.Context) (err error) {
	mark := time.Now()
	for _, collection := range r.collections {
		err = collection.Reconcile(ctx)
		if err != nil {
			err = liberr.Wrap(
				err,
				"collection failed.",
				"object",
				ref.ToKind(collection.Object()))
			return
		}
	}

	r.log.V(3).Info(
		"initial parity.",
		"duration",
		time.Since(mark))

	r.parity = true

	return
}

// Shutdown the collector.
//  1. Close manager stop channel.
//  2. Close watch event coroutine channel.
//  3. Cancel the context.
func (r *Collector) Shutdown() {
	r.log.V(3).Info("shutdown.")
	r.terminate()
	if r.cancel != nil {
		r.cancel()
	}
}

// Terminate coroutines.
func (r *Collector) terminate() {
	defer func() {
		if err := recover(); err != nil {
			r.log.V(4).Info("recovered from panic: ", "err", err)
		}
	}()
	close(r.eventChannel)
}

// Enqueue create model event.
// Used by watch predicates.
// Swallow panic: send on closed channel.
func (r *Collector) Create(m libmodel.Model) {
	defer func() {
		if p := recover(); p != nil {
			r.log.V(4).Info("channel send failed.")
		}
	}()
	r.eventChannel <- ModelEvent{}.Create(m)
}

// Enqueue update model event.
// Used by watch predicates.
// Swallow panic: send on closed channel.
func (r *Collector) Update(m libmodel.Model) {
	defer func() {
		if p := recover(); p != nil {
			r.log.V(4).Info("channel send failed.")
		}
	}()
	r.eventChannel <- ModelEvent{}.Update(m)
}

// Enqueue delete model event.
// Used by watch predicates.
// Swallow panic: send on closed channel.
func (r *Collector) Delete(m libmodel.Model) {
	defer func() {
		if p := recover(); p != nil {
			r.log.V(4).Info("channel send failed.")
		}
	}()
	r.eventChannel <- ModelEvent{}.Delete(m)
}

// Build the k8s manager.
func (r *Collector) buildManager() (err error) {
	provider := r.cluster.(*api.Provider)
	r.manager, err = manager.New(
		ocp.RestCfg(provider, r.secret),
		manager.Options{
			Metrics: server.Options{BindAddress: "0"},
		})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	dsController, err := controller.New(
		r.Name(),
		r.manager,
		controller.Options{
			Reconciler: r,
		})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	for _, collection := range r.collections {
		err = dsController.Watch(
			source.Kind(
				r.manager.GetCache(),
				collection.Object(),
			),
			&handler.EnqueueRequestForObject{},
			collection)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
	}

	return
}

// Build non-cached client.
func (r *Collector) buildClient() (err error) {
	provider := r.cluster.(*api.Provider)
	r.client, err = client.New(
		ocp.RestCfg(provider, r.secret),
		client.Options{
			Scheme: scheme.Scheme,
		})

	return
}

// Apply model events.
func (r *Collector) applyEvents() {
	r.log.V(3).Info("apply started.")
	defer r.log.V(3).Info("apply terminated.")
	for event := range r.eventChannel {
		err := event.Apply(r)
		if err != nil {
			r.log.V(4).Error(
				err, "apply event failed.")
		}
	}
}

// Never called.
func (r *Collector) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	return reconcile.Result{}, nil
}

// Model event.
// Used with `eventChannel`.
type ModelEvent struct {
	// Model the changed.
	model libmodel.Model
	// Action performed on the model:
	//   0x01 Create.
	//   0x02 Update.
	//   0x04 Delete.
	action byte
}

// Apply the change to the DB.
func (r *ModelEvent) Apply(rl *Collector) (err error) {
	tx, err := rl.db.Begin()
	if err != nil {
		return
	}
	defer func() {
		if err != nil {
			_ = tx.End()
		}
	}()
	version := uint64(0)
	if m, cast := r.model.(interface{ ResourceVersion() uint64 }); cast {
		version = m.ResourceVersion()
	}
	switch r.action {
	case 0x01: // Create
		if version > rl.versionThreshold {
			err = tx.Insert(r.model)
			if err != nil {
				return
			}
			rl.log.V(3).Info(
				"model created.",
				ref.ToKind(r.model),
				libmodel.Describe(r.model))
		}
	case 0x02: // Update
		if version > rl.versionThreshold {
			err = tx.Update(r.model)
			if err != nil {
				return
			}
			rl.log.V(3).Info(
				"model updated.",
				ref.ToKind(r.model),
				libmodel.Describe(r.model))
		}
	case 0x04: // Delete
		err = tx.Delete(r.model)
		if err != nil {
			return
		}
		rl.log.V(3).Info(
			"model deleted.",
			ref.ToKind(r.model),
			libmodel.Describe(r.model))
	default:
		return liberr.New(
			"unknown action",
			"action",
			r.action)
	}
	err = tx.Commit()
	if err != nil {
		return
	}

	return
}

// Set the event model and action.
func (r ModelEvent) Create(m libmodel.Model) ModelEvent {
	r.model = m
	r.action = 0x01
	return r
}

// Set the event model and action.
func (r ModelEvent) Update(m libmodel.Model) ModelEvent {
	r.model = m
	r.action = 0x02
	return r
}

// Set the event model and action.
func (r ModelEvent) Delete(m libmodel.Model) ModelEvent {
	r.model = m
	r.action = 0x04
	return r
}
