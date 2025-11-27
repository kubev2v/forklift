package dynamicprovider

import (
	"context"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/base"
	dynamicregistry "github.com/kubev2v/forklift/pkg/controller/provider/web/dynamic"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	"github.com/kubev2v/forklift/pkg/settings"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apiserver/pkg/storage/names"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	Name = "dynamic-provider"
)

var log = logging.WithName(Name)
var Settings = &settings.Settings

// Add creates a new DynamicProvider Controller and adds it to the Manager.
func Add(mgr manager.Manager) error {
	// Initialize the dynamic provider registry
	dynamicregistry.Registry.Initialize(mgr.GetClient())

	reconciler := &Reconciler{
		Reconciler: base.Reconciler{
			EventRecorder: mgr.GetEventRecorderFor(Name),
			Client:        mgr.GetClient(),
			Log:           log,
		},
	}

	cnt, err := controller.New(
		Name,
		mgr,
		controller.Options{
			MaxConcurrentReconciles: Settings.MaxConcurrentReconciles,
			Reconciler:              reconciler,
		})
	if err != nil {
		log.Trace(err)
		return err
	}

	// Watch DynamicProvider CRs
	err = cnt.Watch(
		source.Kind(mgr.GetCache(), &api.DynamicProvider{},
			&handler.TypedEnqueueRequestForObject[*api.DynamicProvider]{},
		))
	if err != nil {
		log.Trace(err)
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &Reconciler{}

// Reconciler reconciles DynamicProvider objects
type Reconciler struct {
	base.Reconciler
}

// Reconcile handles DynamicProvider CR changes
func (r Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (result reconcile.Result, err error) {
	r.Log = logging.WithName(
		names.SimpleNameGenerator.GenerateName(Name+"|"),
		"dynamicprovider",
		request)
	r.Started()
	defer func() {
		result.RequeueAfter = r.Ended(
			result.RequeueAfter,
			err)
		err = nil
	}()

	// Fetch the DynamicProvider CR
	dp := &api.DynamicProvider{}
	err = r.Get(ctx, request.NamespacedName, dp)
	if err != nil {
		if k8serr.IsNotFound(err) {
			// DynamicProvider was deleted - unregister the type
			r.Log.Info("DynamicProvider deleted, unregistering type")
			err = nil
		}
		return
	}

	// DynamicProvider exists - register or update the type
	if dp.DeletionTimestamp == nil {
		err = r.registerDynamicProvider(dp)
	} else {
		err = r.unregisterDynamicProvider(dp)
	}

	return
}

// registerDynamicProvider registers a dynamic provider type in the registry
func (r *Reconciler) registerDynamicProvider(dp *api.DynamicProvider) error {
	refreshInterval := int32(300) // Default 5 minutes
	if dp.Spec.RefreshInterval != nil {
		refreshInterval = *dp.Spec.RefreshInterval
	}

	r.Log.Info("Registering dynamic provider type",
		"type", dp.Spec.Type,
		"displayName", dp.Spec.DisplayName,
		"image", dp.Spec.Image,
		"refreshInterval", refreshInterval)

	dynamicregistry.Registry.RegisterType(
		dp.Spec.Type,
		dp.Spec.DisplayName,
		dp.Spec.Description,
		refreshInterval,
	)

	return nil
}

// unregisterDynamicProvider unregisters a dynamic provider type from the registry
func (r *Reconciler) unregisterDynamicProvider(dp *api.DynamicProvider) error {
	r.Log.Info("Unregistering dynamic provider type",
		"type", dp.Spec.Type)

	dynamicregistry.Registry.Unregister(dp.Spec.Type)

	return nil
}
