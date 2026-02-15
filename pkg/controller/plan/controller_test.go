package plan

import (
	"context"
	"time"

	k8snet "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/base"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeClient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var controllerLog = logging.WithName("controllerTest")

func newScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = core.AddToScheme(scheme)
	_ = k8snet.AddToScheme(scheme)
	_ = api.SchemeBuilder.AddToScheme(scheme)
	return scheme
}

func newReconciler(objects ...runtime.Object) (*Reconciler, client.Client) {
	scheme := newScheme()
	c := fakeClient.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(objects...).
		WithStatusSubresource(&api.Plan{}).
		Build()
	r := &Reconciler{
		Reconciler: base.Reconciler{
			Client: c,
			Log:    controllerLog,
		},
		APIReader: c,
	}
	return r, c
}

// succeededPlan returns a plan with a Succeeded condition so reconcile
// returns early after the finalizer logic (skipping execute/validate).
func succeededPlan(name, namespace string, finalizers ...string) *api.Plan {
	plan := &api.Plan{
		ObjectMeta: meta.ObjectMeta{
			Name:       name,
			Namespace:  namespace,
			Finalizers: finalizers,
		},
	}
	plan.Status.SetCondition(libcnd.Condition{
		Type:   Succeeded,
		Status: True,
	})
	return plan
}

var _ = ginkgo.Describe("Plan Controller Finalizer", func() {
	const (
		planName      = "test-plan"
		planNamespace = "test-ns"
	)
	namespacedName := types.NamespacedName{Name: planName, Namespace: planNamespace}

	ginkgo.Describe("Finalizer addition on normal reconcile", func() {
		ginkgo.It("should add the finalizer when not present", func() {
			plan := succeededPlan(planName, planNamespace)
			reconciler, c := newReconciler(plan)

			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: namespacedName})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			updated := &api.Plan{}
			err = c.Get(context.TODO(), namespacedName, updated)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(controllerutil.ContainsFinalizer(updated, api.PlanFinalizer)).To(gomega.BeTrue())
		})

		ginkgo.It("should not duplicate the finalizer when already present", func() {
			plan := succeededPlan(planName, planNamespace, api.PlanFinalizer)
			reconciler, c := newReconciler(plan)

			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: namespacedName})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			updated := &api.Plan{}
			err = c.Get(context.TODO(), namespacedName, updated)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			count := 0
			for _, f := range updated.Finalizers {
				if f == api.PlanFinalizer {
					count++
				}
			}
			gomega.Expect(count).To(gomega.Equal(1))
		})
	})

	ginkgo.Describe("Finalizer removal on deletion", func() {
		ginkgo.It("should remove the finalizer and allow deletion", func() {
			now := meta.Now()
			plan := &api.Plan{
				ObjectMeta: meta.ObjectMeta{
					Name:              planName,
					Namespace:         planNamespace,
					Finalizers:        []string{api.PlanFinalizer},
					DeletionTimestamp: &now,
				},
			}
			reconciler, c := newReconciler(plan)

			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: namespacedName})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// The fake client auto-deletes the object once all finalizers are
			// removed and DeletionTimestamp is set, so NotFound is expected.
			updated := &api.Plan{}
			err = c.Get(context.TODO(), namespacedName, updated)
			gomega.Expect(k8serr.IsNotFound(err)).To(gomega.BeTrue())
		})

		ginkgo.It("should set requeue when finalizer removal fails", func() {
			now := meta.Now()
			plan := &api.Plan{
				ObjectMeta: meta.ObjectMeta{
					Name:              planName,
					Namespace:         planNamespace,
					Finalizers:        []string{api.PlanFinalizer},
					DeletionTimestamp: &now,
				},
			}
			// APIReader can find the plan, but Client uses a scheme without
			// the Plan type registered so Patch will fail.
			scheme := newScheme()
			reader := fakeClient.NewClientBuilder().
				WithScheme(scheme).
				WithRuntimeObjects(plan).
				Build()
			badScheme := runtime.NewScheme()
			_ = core.AddToScheme(badScheme)
			badClient := fakeClient.NewClientBuilder().
				WithScheme(badScheme).
				Build()
			reconciler := &Reconciler{
				Reconciler: base.Reconciler{
					Client: badClient,
					Log:    controllerLog,
				},
				APIReader: reader,
			}

			result, _ := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: namespacedName})
			gomega.Expect(result.RequeueAfter).To(gomega.Equal(base.SlowReQ))
		})
	})

	ginkgo.Describe("Reconcile of already-deleted plan", func() {
		ginkgo.It("should return without error when plan is not found", func() {
			reconciler, _ := newReconciler()

			result, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: namespacedName})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(result.RequeueAfter).To(gomega.Equal(time.Duration(0)))
		})
	})
})
