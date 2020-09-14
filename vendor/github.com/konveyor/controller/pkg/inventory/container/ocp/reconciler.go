package ocp

import (
	"context"
	liberr "github.com/konveyor/controller/pkg/error"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	"github.com/konveyor/controller/pkg/logging"
	"github.com/konveyor/controller/pkg/ref"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"time"
)

const (
	RetryDelay = time.Second * 5
)

//
// Cluster.
type Cluster interface {
	meta.Object
	// Build the REST configuration
	// for the remote cluster.
	RestCfg(*core.Secret) *rest.Config
}

//
// An OpenShift reconciler.
type Reconciler struct {
	// The cluster CR.
	cluster Cluster
	// DB client.
	db libmodel.DB
	// Credentials secret.
	secret *core.Secret
	// Logger.
	log logging.Logger
	// Collections
	collections []Collection
	// The k8s manager.
	manager manager.Manager
	// The k8s manager/controller `stop` channel.
	stopChannel chan struct{}
	// Model event channel.
	eventChannel chan ModelEvent
	// The model version threshold used to determine if a
	// model event is obsolete. An event (model) with a version
	// lower than the threshold is redundant to changes made
	// during collection reconciliation.
	versionThreshold uint64
	// The reconciler has (initial) consistency.
	consistent bool
	// cancel function.
	cancel func()
}

//
// New reconciler.
func New(
	db libmodel.DB,
	cluster Cluster,
	secret *core.Secret,
	collections ...Collection) *Reconciler {
	//
	log := logging.WithName(cluster.GetName())
	return &Reconciler{
		collections: collections,
		cluster:     cluster,
		secret:      secret,
		log:         log,
		db:          db,
	}
}

//
// The name.
func (r *Reconciler) Name() string {
	return r.cluster.GetName()
}

//
// The owner.
func (r *Reconciler) Owner() meta.Object {
	return r.cluster
}

//
// Get the DB.
func (r *Reconciler) DB() libmodel.DB {
	return r.db
}

//
// Get the Client.
func (r *Reconciler) Client() client.Client {
	return r.manager.GetClient()
}

//
// Reset.
func (r *Reconciler) Reset() {
	r.consistent = false
}

//
// Reconciler has achieved initial consistency.
func (r *Reconciler) HasConsistency() bool {
	return r.consistent
}

//
// Update the versionThreshold
func (r *Reconciler) UpdateThreshold(m libmodel.Model) {
	if m, cast := m.(interface{ ResourceVersion() uint64 }); cast {
		n := m.ResourceVersion()
		if n > r.versionThreshold {
			r.versionThreshold = n
		}
	}
}

//
// Start the reconciler.
func (r *Reconciler) Start() error {
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
					r.log.Trace(err, "retry", RetryDelay)
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

//
// Start details.
//   1. Build and start the manager.
//   2. Reconcile all of the collections.
//   3. Mark consistent.
//   4. Start apply events (coroutine).
func (r *Reconciler) start(ctx context.Context) (err error) {
	r.versionThreshold = 0
	r.eventChannel = make(chan ModelEvent, 100)
	r.stopChannel = make(chan struct{})
	defer func() {
		if err != nil {
			r.terminate()
		}
	}()
	mark := time.Now()
	r.log.Info("Start")
	err = r.buildManager()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	go r.manager.Start(r.stopChannel)
	err = r.reconcileCollections(ctx)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	go r.applyEvents()

	r.log.Info("Started", "duration", time.Since(mark))

	return
}

//
// Reconcile collections.
func (r *Reconciler) reconcileCollections(ctx context.Context) (err error) {
	mark := time.Now()
	for _, collection := range r.collections {
		err = collection.Reconcile(ctx)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
	}

	r.log.Info("Initial consistency.", "duration", time.Since(mark))
	r.consistent = true

	return
}

//
// Shutdown the reconciler.
//   1. Close manager stop channel.
//   2. Close watch event coroutine channel.
//   3. Cancel the context.
func (r *Reconciler) Shutdown() {
	r.log.Info("Shutdown")
	r.terminate()
	if r.cancel != nil {
		r.cancel()
	}
}

//
// Terminate coroutines.
func (r *Reconciler) terminate() {
	defer func() {
		recover()
	}()
	close(r.stopChannel)
	close(r.eventChannel)
}

//
// Enqueue create model event.
// Used by watch predicates.
// Swallow panic: send on closed channel.
func (r *Reconciler) Create(m libmodel.Model) {
	defer func() {
		if p := recover(); p != nil {
			r.log.Info("channel send failed")
		}
	}()
	r.eventChannel <- ModelEvent{}.Create(m)
}

//
// Enqueue update model event.
// Used by watch predicates.
// Swallow panic: send on closed channel.
func (r *Reconciler) Update(m libmodel.Model) {
	defer func() {
		if p := recover(); p != nil {
			r.log.Info("channel send failed")
		}
	}()
	r.eventChannel <- ModelEvent{}.Update(m)
}

//
// Enqueue delete model event.
// Used by watch predicates.
// Swallow panic: send on closed channel.
func (r *Reconciler) Delete(m libmodel.Model) {
	defer func() {
		if p := recover(); p != nil {
			r.log.Info("channel send failed")
		}
	}()
	r.eventChannel <- ModelEvent{}.Delete(m)
}

//
// Build the k8s manager.
func (r *Reconciler) buildManager() (err error) {
	r.manager, err = manager.New(
		r.cluster.RestCfg(r.secret),
		manager.Options{})
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
			&source.Kind{
				Type: collection.Object(),
			},
			&handler.EnqueueRequestForObject{},
			collection)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
	}

	return
}

//
// Apply model events.
func (r *Reconciler) applyEvents() {
	r.log.Info("Apply - started")
	defer r.log.Info("Apply - ended")
	for event := range r.eventChannel {
		err := event.Apply(r)
		if err != nil {
			r.log.Trace(err)
		}
	}
}

//
// Never called.
func (r *Reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	return reconcile.Result{}, nil
}

//
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

//
// Apply the change to the DB.
func (r *ModelEvent) Apply(rl *Reconciler) (err error) {
	tx, err := rl.db.Begin()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	defer func() {
		if err != nil {
			tx.End()
		}
	}()
	version := uint64(0)
	if m, cast := r.model.(interface{ ResourceVersion() uint64 }); cast {
		version = m.ResourceVersion()
	}
	switch r.action {
	case 0x01: // Create
		if version > rl.versionThreshold {
			rl.log.Info("Create", ref.ToKind(r.model), r.model.String())
			err = rl.db.Insert(r.model)
			if err != nil {
				err = liberr.Wrap(err)
				return
			}
		}
	case 0x02: // Update
		if version > rl.versionThreshold {
			rl.log.Info("Update", ref.ToKind(r.model), r.model.String())
			err = rl.db.Update(r.model)
			if err != nil {
				err = liberr.Wrap(err)
				return
			}
		}
	case 0x04: // Delete
		rl.log.Info("Delete", ref.ToKind(r.model), r.model.String())
		err = rl.db.Delete(r.model)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
	default:
		return liberr.New("unknown action")
	}
	err = tx.Commit()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	return
}

//
// Set the event model and action.
func (r ModelEvent) Create(m libmodel.Model) ModelEvent {
	r.model = m
	r.action = 0x01
	return r
}

//
// Set the event model and action.
func (r ModelEvent) Update(m libmodel.Model) ModelEvent {
	r.model = m
	r.action = 0x02
	return r
}

//
// Set the event model and action.
func (r ModelEvent) Delete(m libmodel.Model) ModelEvent {
	r.model = m
	r.action = 0x04
	return r
}
