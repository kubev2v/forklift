package dynamicserver

import (
	"context"
	"strconv"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/base"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apiserver/pkg/storage/names"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	Name                       = "dynamic-provider-server"
	ApplianceManagementEnabled = "ApplianceManagementEnabled"
	FeatureEnabled             = "FeatureEnabled"
)

var log = logging.WithName(Name)

func Add(mgr manager.Manager) error {
	reconciler := &Reconciler{
		Reconciler: base.Reconciler{
			Client:        mgr.GetClient(),
			EventRecorder: mgr.GetEventRecorderFor(Name),
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
	// Primary CR - watch DynamicProviderServer
	err = cnt.Watch(
		source.Kind(mgr.GetCache(), &api.DynamicProviderServer{},
			&handler.TypedEnqueueRequestForObject[*api.DynamicProviderServer]{},
		))
	if err != nil {
		log.Trace(err)
		return err
	}
	return nil
}

var _ reconcile.Reconciler = &Reconciler{}

type Reconciler struct {
	base.Reconciler
}

func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (result reconcile.Result, err error) {
	r.Log = logging.WithName(
		names.SimpleNameGenerator.GenerateName(Name+"|"),
		"server",
		request)
	r.Started()
	defer func() {
		result.RequeueAfter = r.Ended(
			result.RequeueAfter,
			err)
		err = nil
	}()

	server := &api.DynamicProviderServer{}
	err = r.Get(ctx, request.NamespacedName, server)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			r.Log.Info("Dynamic provider server deleted.")
			err = nil
		}
		return
	}

	defer func() {
		r.Log.V(2).Info("Conditions.", "all", server.Status.Conditions)
	}()

	if server.DeletionTimestamp.IsZero() {
		err = r.AddFinalizer(ctx, server)
		if err != nil {
			return
		}
		err = r.Deploy(ctx, server)
		if err != nil {
			return
		}
	} else {
		err = r.Teardown(ctx, server)
		if err != nil {
			return
		}
		err = r.RemoveFinalizer(ctx, server)
		if err != nil {
			return
		}
	}
	err = r.Status().Update(ctx, server)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	// Done.
	return
}

func (r *Reconciler) AddFinalizer(ctx context.Context, server *api.DynamicProviderServer) (err error) {
	patch := client.MergeFrom(server.DeepCopy())
	if controllerutil.AddFinalizer(server, api.DynamicProviderFinalizer) {
		err = r.Patch(ctx, server, patch)
		if err != nil {
			err = liberr.Wrap(err)
			r.Log.Error(err, "failed to add finalizer", "server", server.Name, "namespace", server.Namespace)
			return
		}
	}
	return
}

func (r *Reconciler) RemoveFinalizer(ctx context.Context, server *api.DynamicProviderServer) (err error) {
	patch := client.MergeFrom(server.DeepCopy())
	if controllerutil.RemoveFinalizer(server, api.DynamicProviderFinalizer) {
		err = r.Patch(ctx, server, patch)
		if err != nil {
			err = liberr.Wrap(err)
			r.Log.Error(err, "failed to remove finalizer", "server", server.Name, "namespace", server.Namespace)
			return
		}
	}
	return
}
func (r *Reconciler) Deploy(ctx context.Context, server *api.DynamicProviderServer) (err error) {
	// Get the DynamicProvider to get the provider type
	dynamicProvider := &api.DynamicProvider{}
	err = r.Get(ctx, types.NamespacedName{
		Name:      server.Spec.DynamicProviderRef.Name,
		Namespace: server.Spec.DynamicProviderRef.Namespace,
	}, dynamicProvider)
	if err != nil {
		r.Log.Error(err, "Failed to get DynamicProvider", "name", server.Spec.DynamicProviderRef.Name)
		return
	}

	// Get the Provider to copy fields from it
	provider := &api.Provider{}
	err = r.Get(ctx, types.NamespacedName{
		Name:      server.Spec.ProviderRef.Name,
		Namespace: server.Spec.ProviderRef.Namespace,
	}, provider)
	if err != nil {
		r.Log.Error(err, "Failed to get Provider", "name", server.Spec.ProviderRef.Name)
		return
	}

	// Sync spec fields from DynamicProvider and Provider
	// Create patch base BEFORE syncing
	patchBase := server.DeepCopy()
	needsUpdate := r.syncServerSpec(server, dynamicProvider, provider)
	if needsUpdate {
		r.Log.Info("Syncing DynamicProviderServer spec with defaults from DynamicProvider and Provider",
			"server", server.Name,
			"image", server.Spec.Image)
		patch := client.MergeFrom(patchBase)
		err = r.Patch(ctx, server, patch)
		if err != nil {
			err = liberr.Wrap(err)
			r.Log.Error(err, "Failed to patch DynamicProviderServer spec")
			return
		}
	}

	providerType := dynamicProvider.Spec.Type

	build := Builder{
		Server:       server,
		Provider:     provider,
		ProviderType: providerType,
	}
	ensure := Ensurer{
		Client: r.Client,
		Server: server,
		Log:    r.Log,
	}

	// Create PVCs for all storage volumes in the spec
	var pvcs []*v1.PersistentVolumeClaim
	if len(server.Spec.Storages) > 0 {
		pvcTemplates := build.PersistentVolumeClaims()
		for _, pvcTemplate := range pvcTemplates {
			pvc, pvcErr := ensure.PersistentVolumeClaim(ctx, pvcTemplate)
			if pvcErr != nil {
				err = pvcErr
				return
			}
			pvcs = append(pvcs, pvc)
		}
		// Store PVC references in status
		server.Status.PVCs = nil
		for _, pvc := range pvcs {
			server.Status.PVCs = append(server.Status.PVCs, v1.ObjectReference{
				Kind:      "PersistentVolumeClaim",
				Namespace: pvc.Namespace,
				Name:      pvc.Name,
			})
		}
	}

	deployment := build.Deployment(pvcs)
	err = ensure.Deployment(ctx, deployment)
	if err != nil {
		return
	}
	service := build.Service()
	service, err = ensure.Service(ctx, service)
	if err != nil {
		return
	}
	if r.managementEndpoints(deployment) {
		server.Status.SetCondition(
			libcnd.Condition{
				Type:     ApplianceManagementEnabled,
				Status:   libcnd.True,
				Reason:   FeatureEnabled,
				Category: libcnd.Advisory,
				Message:  "Dynamic provider server management endpoints are enabled.",
			})
	}
	server.Status.Service = &v1.ObjectReference{
		Kind:      "Service",
		Namespace: service.Namespace,
		Name:      service.Name,
	}
	server.Status.ProviderType = providerType
	server.Status.Phase = libcnd.Ready
	return
}

// syncServerSpec syncs the DynamicProviderServer spec with defaults from DynamicProvider and Provider.
// Returns true if the server spec was updated.
func (r *Reconciler) syncServerSpec(server *api.DynamicProviderServer, dynamicProvider *api.DynamicProvider, provider *api.Provider) bool {
	updated := false

	// Copy Storages from DynamicProvider if not already set
	if len(server.Spec.Storages) == 0 && len(dynamicProvider.Spec.Storages) > 0 {
		r.Log.V(1).Info("Copying Storages from DynamicProvider to DynamicProviderServer")
		server.Spec.Storages = make([]api.StorageSpec, len(dynamicProvider.Spec.Storages))
		copy(server.Spec.Storages, dynamicProvider.Spec.Storages)
		updated = true
	}

	// Copy Resources from DynamicProvider if not already set
	if server.Spec.Resources == nil && dynamicProvider.Spec.Resources != nil {
		r.Log.V(1).Info("Copying Resources from DynamicProvider to DynamicProviderServer")
		server.Spec.Resources = dynamicProvider.Spec.Resources.DeepCopy()
		updated = true
	}

	// Copy Env from DynamicProvider if not already set (merge, don't replace)
	if len(server.Spec.Env) == 0 && len(dynamicProvider.Spec.Env) > 0 {
		r.Log.V(1).Info("Copying Env from DynamicProvider to DynamicProviderServer")
		server.Spec.Env = make([]v1.EnvVar, len(dynamicProvider.Spec.Env))
		for i := range dynamicProvider.Spec.Env {
			server.Spec.Env[i] = *dynamicProvider.Spec.Env[i].DeepCopy()
		}
		updated = true
	}

	// Copy Volumes from Provider if not already set
	// These are existing volume sources (NFS, PVC, ConfigMap, etc.) - NOT created by controller
	if len(server.Spec.Volumes) == 0 && len(provider.Spec.Volumes) > 0 {
		r.Log.V(1).Info("Copying Volumes from Provider to DynamicProviderServer")
		server.Spec.Volumes = make([]api.ProviderVolume, len(provider.Spec.Volumes))
		copy(server.Spec.Volumes, provider.Spec.Volumes)
		updated = true
	}

	// Copy NodeSelector from Provider if not already set
	if len(server.Spec.NodeSelector) == 0 && len(provider.Spec.ServerNodeSelector) > 0 {
		r.Log.V(1).Info("Copying ServerNodeSelector from Provider to DynamicProviderServer")
		server.Spec.NodeSelector = make(map[string]string)
		for k, v := range provider.Spec.ServerNodeSelector {
			server.Spec.NodeSelector[k] = v
		}
		updated = true
	}

	// Copy Affinity from Provider if not already set
	if server.Spec.Affinity == nil && provider.Spec.ServerAffinity != nil {
		r.Log.V(1).Info("Copying ServerAffinity from Provider to DynamicProviderServer")
		server.Spec.Affinity = provider.Spec.ServerAffinity.DeepCopy()
		updated = true
	}

	// Copy Image from DynamicProvider if not already set
	if server.Spec.Image == "" && dynamicProvider.Spec.Image != "" {
		r.Log.V(1).Info("Copying Image from DynamicProvider to DynamicProviderServer")
		server.Spec.Image = dynamicProvider.Spec.Image
		updated = true
	}

	// Copy ImagePullPolicy from DynamicProvider if not already set
	if server.Spec.ImagePullPolicy == nil && dynamicProvider.Spec.ImagePullPolicy != nil {
		r.Log.V(1).Info("Copying ImagePullPolicy from DynamicProvider to DynamicProviderServer")
		policy := *dynamicProvider.Spec.ImagePullPolicy
		server.Spec.ImagePullPolicy = &policy
		updated = true
	}

	// Copy Port from DynamicProvider if not already set
	if server.Spec.Port == nil && dynamicProvider.Spec.Port != nil {
		r.Log.V(1).Info("Copying Port from DynamicProvider to DynamicProviderServer")
		port := *dynamicProvider.Spec.Port
		server.Spec.Port = &port
		updated = true
	}

	// Copy RefreshInterval from DynamicProvider if not already set
	if server.Spec.RefreshInterval == nil && dynamicProvider.Spec.RefreshInterval != nil {
		r.Log.V(1).Info("Copying RefreshInterval from DynamicProvider to DynamicProviderServer")
		interval := *dynamicProvider.Spec.RefreshInterval
		server.Spec.RefreshInterval = &interval
		updated = true
	}

	// Copy ImagePullSecrets from DynamicProvider if not already set
	if len(server.Spec.ImagePullSecrets) == 0 && len(dynamicProvider.Spec.ImagePullSecrets) > 0 {
		r.Log.V(1).Info("Copying ImagePullSecrets from DynamicProvider to DynamicProviderServer")
		server.Spec.ImagePullSecrets = make([]v1.LocalObjectReference, len(dynamicProvider.Spec.ImagePullSecrets))
		copy(server.Spec.ImagePullSecrets, dynamicProvider.Spec.ImagePullSecrets)
		updated = true
	}

	return updated
}

func (r *Reconciler) Teardown(ctx context.Context, server *api.DynamicProviderServer) (err error) {
	// Check if the Provider still exists
	provider := &api.Provider{}
	err = r.Get(ctx, types.NamespacedName{
		Name:      server.Spec.ProviderRef.Name,
		Namespace: server.Spec.ProviderRef.Namespace,
	}, provider)
	if err != nil && !k8serrors.IsNotFound(err) {
		log.Error(err, "Failed to get provider.")
		err = liberr.Wrap(err)
		return
	}

	// Get provider type from status or DynamicProvider
	providerType := server.Status.ProviderType
	if providerType == "" {
		// If not in status, get from DynamicProvider
		dynamicProvider := &api.DynamicProvider{}
		dpErr := r.Get(ctx, types.NamespacedName{
			Name:      server.Spec.DynamicProviderRef.Name,
			Namespace: server.Spec.DynamicProviderRef.Namespace,
		}, dynamicProvider)
		if dpErr == nil {
			providerType = dynamicProvider.Spec.Type
		} else if !k8serrors.IsNotFound(dpErr) {
			log.Error(dpErr, "Failed to get DynamicProvider.")
			err = liberr.Wrap(dpErr)
			return
		}
	}

	// Clean up resources
	del := Deleter{
		Server:       server,
		ProviderType: providerType,
		Client:       r.Client,
		Log:          r.Log,
	}
	err = del.Deployment(ctx)
	if err != nil {
		return
	}
	err = del.Service(ctx)
	if err != nil {
		return
	}
	err = del.PersistentVolumeClaim(ctx)
	if err != nil {
		return
	}
	return
}

func (r *Reconciler) managementEndpoints(deployment *appsv1.Deployment) bool {
	for _, container := range deployment.Spec.Template.Spec.Containers {
		for _, env := range container.Env {
			if env.Name == ApplianceEndpoints {
				return env.Value == strconv.FormatBool(true)
			}
		}
	}
	return false
}
