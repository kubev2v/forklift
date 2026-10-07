package copyappliancetemplate

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
		source.Kind(mgr.GetCache(), &api.CopyApplianceTemplate{}, &handler.TypedEnqueueRequestForObject[*api.CopyApplianceTemplate]{}, &CopyApplianceTemplatePredicate{}))
	if err != nil {
		return err
	}
	return cnt.Watch(
		source.Kind(mgr.GetCache(), &core.Pod{}, copyApplianceTemplateForBuildPodMapper(),
			predicate.NewTypedPredicateFuncs(func(obj *core.Pod) bool {
				_, ok := obj.Labels[api.LabelCopyApplianceTemplate]
				return ok
			})))
}

type Reconciler struct {
	base.Reconciler
	Scheme *runtime.Scheme
}

func (r Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (result reconcile.Result, err error) {
	r.Log = logging.WithName(names.SimpleNameGenerator.GenerateName(Name+"|"), "copyApplianceTemplate", request)
	r.Started()
	defer func() {
		result.RequeueAfter = r.Ended(result.RequeueAfter, err)
		err = nil
	}()

	copyApplianceTemplate := &api.CopyApplianceTemplate{}
	err = r.Get(ctx, request.NamespacedName, copyApplianceTemplate)
	if err != nil {
		if k8serr.IsNotFound(err) {
			err = nil
		}
		return
	}

	if !copyApplianceTemplate.DeletionTimestamp.IsZero() {
		return r.finalize(ctx, copyApplianceTemplate)
	}

	if copyApplianceTemplate.Status.Phase == api.CopyApplianceTemplatePhaseSucceeded {
		if copyApplianceTemplate.Status.ObservedGeneration >= copyApplianceTemplate.Generation {
			return
		}
		copyApplianceTemplate.Status.Phase = api.CopyApplianceTemplatePhaseRunning
		copyApplianceTemplate.Status.Stage = api.StageEnsureTemplate
	}
	if copyApplianceTemplate.Status.Phase == api.CopyApplianceTemplatePhaseFailed {
		if copyApplianceTemplate.Status.ObservedGeneration >= copyApplianceTemplate.Generation {
			return
		}
		copyApplianceTemplate.Status.Phase = api.CopyApplianceTemplatePhaseRunning
		copyApplianceTemplate.Status.Stage = api.StageEnsureTemplate
		log.Info("retrying failed copy appliance template after spec change",
			"copyApplianceTemplate", copyApplianceTemplate.Name,
			"generation", copyApplianceTemplate.Generation,
		)
	}

	if !controllerutil.ContainsFinalizer(copyApplianceTemplate, api.CopyApplianceTemplateFinalizer) {
		controllerutil.AddFinalizer(copyApplianceTemplate, api.CopyApplianceTemplateFinalizer)
		if err = r.Update(ctx, copyApplianceTemplate); err != nil {
			return
		}
	}

	copyApplianceTemplate.Status.BeginStagingConditions()
	if err = r.validate(ctx, copyApplianceTemplate); err != nil {
		r.fail(copyApplianceTemplate, err)
		copyApplianceTemplate.Status.EndStagingConditions()
		r.Record(copyApplianceTemplate, copyApplianceTemplate.Status.Conditions)
		copyApplianceTemplate.Status.ObservedGeneration = copyApplianceTemplate.Generation
		_ = r.Status().Update(ctx, copyApplianceTemplate)
		return
	}

	runner := &Runner{context: &CopyApplianceTemplateContext{Client: r.Client, Scheme: r.Scheme, CopyApplianceTemplate: copyApplianceTemplate}}
	done, pipeErr := runner.Run(ctx)
	if pipeErr != nil {
		r.fail(copyApplianceTemplate, pipeErr)
	} else if done {
		copyApplianceTemplate.Status.Phase = api.CopyApplianceTemplatePhaseSucceeded
		copyApplianceTemplate.Status.SetCondition(libcnd.Condition{
			Type:     libcnd.Ready,
			Status:   libcnd.True,
			Category: libcnd.Required,
			Message:  "Copy appliance template is ready.",
		})
	} else {
		result.RequeueAfter = base.SlowReQ
	}

	if copyApplianceTemplate.Status.Message == "" {
		switch copyApplianceTemplate.Status.Stage {
		case api.StageEnsurePrerequisites, api.StageEnsureTemplate:
			copyApplianceTemplate.Status.Message = "Ensuring template prerequisites and reuse."
		case api.StageBuildAndUpload:
			copyApplianceTemplate.Status.Message = "Building and uploading template."
		case api.StageTemplateFinished:
			copyApplianceTemplate.Status.Message = "Copy appliance template is ready."
		default:
			copyApplianceTemplate.Status.Message = "Reconciling copy appliance template."
		}
	}
	copyApplianceTemplate.Status.EndStagingConditions()
	r.Record(copyApplianceTemplate, copyApplianceTemplate.Status.Conditions)
	copyApplianceTemplate.Status.ObservedGeneration = copyApplianceTemplate.Generation
	err = r.Status().Update(ctx, copyApplianceTemplate)
	return
}

func (r Reconciler) fail(copyApplianceTemplate *api.CopyApplianceTemplate, cause error) {
	copyApplianceTemplate.Status.Phase = api.CopyApplianceTemplatePhaseFailed
	now := meta.Now()
	copyApplianceTemplate.Status.CompletionTime = &now
	msg := "The copy appliance template has failed."
	if cause != nil {
		msg = fmt.Sprintf("%s %s", msg, cause.Error())
	}
	copyApplianceTemplate.Status.Message = msg
	copyApplianceTemplate.Status.SetCondition(libcnd.Condition{
		Type:     api.CopyApplianceTemplateFailed,
		Status:   libcnd.True,
		Category: libcnd.Critical,
		Message:  msg,
	})
}

func (r Reconciler) finalize(ctx context.Context, copyApplianceTemplate *api.CopyApplianceTemplate) (reconcile.Result, error) {
	if !controllerutil.ContainsFinalizer(copyApplianceTemplate, api.CopyApplianceTemplateFinalizer) {
		return reconcile.Result{}, nil
	}
	tc := &CopyApplianceTemplateContext{Client: r.Client, Scheme: r.Scheme, CopyApplianceTemplate: copyApplianceTemplate}
	if pctx, err := tc.providerContext(ctx); err == nil {
		defer func() { _ = pctx.Client.Close(ctx) }()
		if ref, findErr := pctx.Client.FindVM(ctx, copyApplianceTemplate.Spec.Folder, copyApplianceTemplate.Spec.TemplateName, true); findErr == nil {
			_ = libvsphere.Destroy(ctx, ref.VM)
		}
	}
	if err := tc.deleteBuildPod(ctx); err != nil {
		return reconcile.Result{}, err
	}
	controllerutil.RemoveFinalizer(copyApplianceTemplate, api.CopyApplianceTemplateFinalizer)
	if err := r.Update(ctx, copyApplianceTemplate); err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

type CopyApplianceTemplatePredicate struct {
	predicate.TypedFuncs[*api.CopyApplianceTemplate]
}

func (r CopyApplianceTemplatePredicate) Create(e event.TypedCreateEvent[*api.CopyApplianceTemplate]) bool {
	libref.Mapper.Create(event.CreateEvent{Object: e.Object})
	return true
}

func (r CopyApplianceTemplatePredicate) Update(e event.TypedUpdateEvent[*api.CopyApplianceTemplate]) bool {
	object := e.ObjectNew
	changed := object.Status.ObservedGeneration < object.Generation
	if changed {
		libref.Mapper.Update(event.UpdateEvent{
			ObjectOld: e.ObjectOld,
			ObjectNew: e.ObjectNew,
		})
	}
	return changed || object.Status.Phase == api.CopyApplianceTemplatePhaseRunning
}

func (r CopyApplianceTemplatePredicate) Delete(e event.TypedDeleteEvent[*api.CopyApplianceTemplate]) bool {
	libref.Mapper.Delete(event.DeleteEvent{Object: e.Object})
	return true
}

func copyApplianceTemplateForBuildPodMapper() handler.TypedEventHandler[*core.Pod, reconcile.Request] {
	return handler.TypedEnqueueRequestsFromMapFunc(func(_ context.Context, pod *core.Pod) []reconcile.Request {
		name := pod.Labels[api.LabelCopyApplianceTemplate]
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

func (r Reconciler) validate(ctx context.Context, copyApplianceTemplate *api.CopyApplianceTemplate) error {
	if copyApplianceTemplate.Spec.Provider.Name == "" {
		return liberr.New("spec.provider.name is required")
	}
	if copyApplianceTemplate.Spec.BaseDisk.ContainerImage == "" {
		copyApplianceTemplate.Spec.BaseDisk.ContainerImage = Settings.BaseDiskContainerImage
	}
	if copyApplianceTemplate.Spec.BaseDisk.ContainerImage == "" {
		return liberr.New("spec.baseDisk.containerImage is required")
	}
	if copyApplianceTemplate.Spec.TemplateName == "" {
		return liberr.New("spec.templateName is required")
	}
	if copyApplianceTemplate.Spec.Datastore == "" {
		return liberr.New("spec.datastore is required")
	}
	if copyApplianceTemplate.Spec.Folder == "" {
		return liberr.New("spec.folder is required")
	}
	if copyApplianceTemplate.Spec.Network == "" {
		return liberr.New("spec.network is required")
	}
	provider := &api.Provider{}
	err := r.Get(ctx, types.NamespacedName{
		Namespace: copyApplianceTemplate.Spec.Provider.Namespace,
		Name:      copyApplianceTemplate.Spec.Provider.Name,
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
