package toehold

import (
	"context"
	"fmt"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/base"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	libref "github.com/kubev2v/forklift/pkg/lib/ref"
	libvsphere "github.com/kubev2v/forklift/pkg/lib/vsphere"
	core "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apiserver/pkg/storage/names"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

func Add(mgr manager.Manager) error {
	reconciler := &Reconciler{
		Reconciler: base.Reconciler{
			EventRecorder: mgr.GetEventRecorderFor(Name),
			Client:        mgr.GetClient(),
			Log:           log,
		},
		Scheme: mgr.GetScheme(),
	}
	cnt, err := controller.New(Name, mgr, controller.Options{
		Reconciler:              reconciler,
		MaxConcurrentReconciles: Settings.MaxConcurrentReconciles,
	})
	if err != nil {
		return err
	}
	err = cnt.Watch(
		source.Kind(mgr.GetCache(), &api.ToeholdTemplate{}, &handler.TypedEnqueueRequestForObject[*api.ToeholdTemplate]{}, &ToeholdPredicate{}))
	if err != nil {
		return err
	}
	return cnt.Watch(
		source.Kind(mgr.GetCache(), &core.Pod{}, toeholdForBuildPodMapper(),
			predicate.NewTypedPredicateFuncs(func(obj *core.Pod) bool {
				_, ok := obj.Labels[api.LabelToehold]
				return ok
			})))
}

type Reconciler struct {
	base.Reconciler
	Scheme *runtime.Scheme
}

func (r Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (result reconcile.Result, err error) {
	r.Log = logging.WithName(names.SimpleNameGenerator.GenerateName(Name+"|"), "toeholdTemplate", request)
	r.Started()
	defer func() {
		result.RequeueAfter = r.Ended(result.RequeueAfter, err)
		err = nil
	}()

	toehold := &api.ToeholdTemplate{}
	err = r.Get(ctx, request.NamespacedName, toehold)
	if err != nil {
		if k8serr.IsNotFound(err) {
			err = nil
		}
		return
	}

	if !toehold.DeletionTimestamp.IsZero() {
		return r.finalize(ctx, toehold)
	}

	if toehold.Status.Phase == api.ToeholdTemplatePhaseSucceeded {
		if toehold.Status.ObservedGeneration >= toehold.Generation {
			return
		}
		toehold.Status.Phase = api.ToeholdTemplatePhaseRunning
		toehold.Status.Stage = api.StageEnsureTemplate
	}
	if toehold.Status.Phase == api.ToeholdTemplatePhaseFailed {
		if toehold.Status.ObservedGeneration >= toehold.Generation {
			return
		}
		toehold.Status.Phase = api.ToeholdTemplatePhaseRunning
		toehold.Status.Stage = api.StageEnsureTemplate
		log.Info("retrying failed toehold template after spec change",
			"toeholdTemplate", toehold.Name,
			"generation", toehold.Generation,
		)
	}

	if !controllerutil.ContainsFinalizer(toehold, api.ToeholdTemplateFinalizer) {
		controllerutil.AddFinalizer(toehold, api.ToeholdTemplateFinalizer)
		if err = r.Update(ctx, toehold); err != nil {
			return
		}
	}

	toehold.Status.BeginStagingConditions()
	if err = r.validate(ctx, toehold); err != nil {
		r.fail(toehold, err)
		toehold.Status.EndStagingConditions()
		r.Record(toehold, toehold.Status.Conditions)
		toehold.Status.ObservedGeneration = toehold.Generation
		_ = r.Status().Update(ctx, toehold)
		return
	}

	runner := &Runner{context: &ToeholdContext{Client: r.Client, Scheme: r.Scheme, Toehold: toehold}}
	done, pipeErr := runner.Run(ctx)
	if pipeErr != nil {
		r.fail(toehold, pipeErr)
	} else if done {
		toehold.Status.Phase = api.ToeholdTemplatePhaseSucceeded
		toehold.Status.SetCondition(libcnd.Condition{
			Type:     libcnd.Ready,
			Status:   libcnd.True,
			Category: libcnd.Required,
			Message:  "Toehold template is ready.",
		})
	} else {
		result.RequeueAfter = base.SlowReQ
	}

	if toehold.Status.Message == "" {
		switch toehold.Status.Stage {
		case api.StageEnsurePrerequisites, api.StageEnsureTemplate:
			toehold.Status.Message = "Ensuring template prerequisites and reuse."
		case api.StageBuildAndUpload:
			toehold.Status.Message = "Building and uploading template."
		case api.StageToeholdFinished:
			toehold.Status.Message = "Toehold template is ready."
		default:
			toehold.Status.Message = "Reconciling toehold template."
		}
	}
	toehold.Status.EndStagingConditions()
	r.Record(toehold, toehold.Status.Conditions)
	toehold.Status.ObservedGeneration = toehold.Generation
	err = r.Status().Update(ctx, toehold)
	return
}

func (r Reconciler) fail(toehold *api.ToeholdTemplate, cause error) {
	toehold.Status.Phase = api.ToeholdTemplatePhaseFailed
	now := meta.Now()
	toehold.Status.CompletionTime = &now
	msg := "The toehold template has failed."
	if cause != nil {
		msg = fmt.Sprintf("%s %s", msg, cause.Error())
	}
	toehold.Status.Message = msg
	toehold.Status.SetCondition(libcnd.Condition{
		Type:     api.ToeholdTemplateFailed,
		Status:   libcnd.True,
		Category: libcnd.Critical,
		Message:  msg,
	})
}

func (r Reconciler) finalize(ctx context.Context, toehold *api.ToeholdTemplate) (reconcile.Result, error) {
	if !controllerutil.ContainsFinalizer(toehold, api.ToeholdTemplateFinalizer) {
		return reconcile.Result{}, nil
	}
	tc := &ToeholdContext{Client: r.Client, Scheme: r.Scheme, Toehold: toehold}
	if pctx, err := tc.providerContext(ctx); err == nil {
		defer func() { _ = pctx.Client.Close(ctx) }()
		if ref, findErr := pctx.Client.FindVM(ctx, toehold.Spec.Folder, toehold.Spec.TemplateName, true); findErr == nil {
			_ = libvsphere.Destroy(ctx, ref.VM)
		}
	}
	if err := tc.deleteBuildPod(ctx); err != nil {
		return reconcile.Result{}, err
	}
	controllerutil.RemoveFinalizer(toehold, api.ToeholdTemplateFinalizer)
	if err := r.Update(ctx, toehold); err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

type ToeholdPredicate struct {
	predicate.TypedFuncs[*api.ToeholdTemplate]
}

func (r ToeholdPredicate) Create(e event.TypedCreateEvent[*api.ToeholdTemplate]) bool {
	libref.Mapper.Create(event.CreateEvent{Object: e.Object})
	return true
}

func (r ToeholdPredicate) Update(e event.TypedUpdateEvent[*api.ToeholdTemplate]) bool {
	object := e.ObjectNew
	changed := object.Status.ObservedGeneration < object.Generation
	if changed {
		libref.Mapper.Update(event.UpdateEvent{
			ObjectOld: e.ObjectOld,
			ObjectNew: e.ObjectNew,
		})
	}
	return changed || object.Status.Phase == api.ToeholdTemplatePhaseRunning
}

func (r ToeholdPredicate) Delete(e event.TypedDeleteEvent[*api.ToeholdTemplate]) bool {
	libref.Mapper.Delete(event.DeleteEvent{Object: e.Object})
	return true
}

func toeholdForBuildPodMapper() handler.TypedEventHandler[*core.Pod, reconcile.Request] {
	return handler.TypedEnqueueRequestsFromMapFunc(func(_ context.Context, pod *core.Pod) []reconcile.Request {
		name := pod.Labels[api.LabelToehold]
		if name == "" {
			return nil
		}
		return []reconcile.Request{{
			NamespacedName: types.NamespacedName{
				Namespace: pod.Namespace,
				Name:      name,
			},
		}}
	})
}

func (r Reconciler) validate(ctx context.Context, toehold *api.ToeholdTemplate) error {
	if toehold.Spec.Provider.Name == "" {
		return liberr.New("spec.provider.name is required")
	}
	if toehold.Spec.BaseDisk.ContainerImage == "" {
		toehold.Spec.BaseDisk.ContainerImage = Settings.BaseDiskContainerImage
	}
	if toehold.Spec.BaseDisk.ContainerImage == "" {
		return liberr.New("spec.baseDisk.containerImage is required")
	}
	if toehold.Spec.TemplateName == "" {
		return liberr.New("spec.templateName is required")
	}
	if toehold.Spec.Datastore == "" {
		return liberr.New("spec.datastore is required")
	}
	if toehold.Spec.Folder == "" {
		return liberr.New("spec.folder is required")
	}
	if toehold.Spec.Network == "" {
		return liberr.New("spec.network is required")
	}
	provider := &api.Provider{}
	err := r.Get(ctx, types.NamespacedName{
		Namespace: toehold.Spec.Provider.Namespace,
		Name:      toehold.Spec.Provider.Name,
	}, provider)
	if err != nil {
		return liberr.Wrap(err)
	}
	if provider.Type() != api.VSphere {
		return liberr.New("spec.provider must reference a vSphere provider")
	}
	secret := &core.Secret{}
	err = r.Get(ctx, types.NamespacedName{
		Namespace: provider.Spec.Secret.Namespace,
		Name:      provider.Spec.Secret.Name,
	}, secret)
	if err != nil {
		if k8serr.IsNotFound(err) {
			return liberr.New(fmt.Sprintf("provider secret %s not found", provider.Spec.Secret.Name))
		}
		return liberr.Wrap(err)
	}
	return nil
}
