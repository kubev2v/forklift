/*
Copyright 2019 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package plan

import (
	"context"
	"github.com/konveyor/virt-controller/pkg/settings"

	"github.com/konveyor/controller/pkg/logging"
	api "github.com/konveyor/virt-controller/pkg/apis/virt/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logging.WithName("plan")

//
// Application settings.
var Settings = &settings.Settings

//
// Creates a new Plan Controller and adds it to the Manager.
func Add(mgr manager.Manager) error {
	reconciler := &Reconciler{
		Client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
	}
	cnt, err := controller.New(
		"plan-controller",
		mgr,
		controller.Options{
			Reconciler: reconciler,
		})
	if err != nil {
		log.Trace(err)
		return err
	}
	err = cnt.Watch(&source.Kind{
		Type: &api.Plan{}},
		&handler.EnqueueRequestForObject{},
		&PlanPredicate{},
	)
	if err != nil {
		log.Trace(err)
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &Reconciler{}

//
// Reconciles a Plan object.
type Reconciler struct {
	client.Client
	scheme *runtime.Scheme
}

//
// Reconcile a Plan CR.
func (r *Reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	var err error
	log.Reset()

	// Fetch the CR.
	plan := &api.Plan{}
	err = r.Get(context.TODO(), request.NamespacedName, plan)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		log.Trace(err)
		return reconcile.Result{}, err
	}

	// Validations.
	err = r.validate(plan)
	if err != nil {
		log.Trace(err)
		return reconcile.Result{Requeue: true}, nil
	}

	// Ready condition.
	if !plan.Status.HasBlockerCondition() {
		plan.Status.SetReady(true, ReadyMessage)
	}

	// Apply changes.
	//plan.MarkReconciled()
	err = r.Update(context.TODO(), plan)
	if err != nil {
		log.Trace(err)
		return reconcile.Result{Requeue: true}, nil
	}

	// Done
	return reconcile.Result{}, nil
}
