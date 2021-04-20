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

package migration

import (
	"context"
	libcnd "github.com/konveyor/controller/pkg/condition"
	"github.com/konveyor/controller/pkg/logging"
	libref "github.com/konveyor/controller/pkg/ref"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	"github.com/konveyor/forklift-controller/pkg/controller/base"
	"github.com/konveyor/forklift-controller/pkg/settings"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apiserver/pkg/storage/names"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	// Name.
	Name = "migration"
)

//
// Package logger.
var log = logging.WithName(Name)

//
// Application settings.
var Settings = &settings.Settings

//
// Creates a new Migration Controller and adds it to the Manager.
func Add(mgr manager.Manager) error {
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
			Reconciler: reconciler,
		})
	if err != nil {
		log.Trace(err)
		return err
	}
	// Primary CR.
	err = cnt.Watch(
		&source.Kind{
			Type: &api.Migration{},
		},
		&handler.EnqueueRequestForObject{},
		&MigrationPredicate{})
	if err != nil {
		log.Trace(err)
		return err
	}
	// References.
	err = cnt.Watch(
		&source.Kind{
			Type: &api.Plan{},
		},
		libref.Handler(&api.Migration{}),
		&PlanPredicate{})
	if err != nil {
		log.Trace(err)
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &Reconciler{}

//
// Reconciles a Migration object.
type Reconciler struct {
	base.Reconciler
}

//
// Reconcile a Migration CR.
// Note: Must not a pointer receiver to ensure that the
// logger and other state is not shared.
func (r Reconciler) Reconcile(request reconcile.Request) (result reconcile.Result, err error) {
	r.Log = logging.WithName(
		names.SimpleNameGenerator.GenerateName(Name+"|"),
		"map",
		request)
	r.Started()
	defer func() {
		result.RequeueAfter = r.Ended(
			result.RequeueAfter,
			err)
		err = nil
	}()

	// Fetch the CR.
	migration := &api.Migration{}
	err = r.Get(context.TODO(), request.NamespacedName, migration)
	if err != nil {
		if k8serr.IsNotFound(err) {
			r.Log.Info("migration deleted.")
			err = nil
		}
		return
	}
	defer func() {
		r.Log.V(1).Info("Conditions.", "all", migration.Status.Conditions)
	}()

	// Detected completed.
	if migration.Status.MarkedCompleted() {
		return
	}

	// Begin staging conditions.
	migration.Status.BeginStagingConditions()

	// Validations.
	plan, err := r.validate(migration)
	if err != nil {
		return
	}

	// Reflect plan.
	r.reflectPlan(plan, migration)

	// Ready condition.
	if !migration.Status.HasBlockerCondition() {
		migration.Status.SetCondition(libcnd.Condition{
			Type:     libcnd.Ready,
			Status:   True,
			Category: Required,
			Message:  "The migration is ready.",
		})
	}

	// End staging conditions.
	migration.Status.EndStagingConditions()

	// Apply changes.
	migration.Status.ObservedGeneration = migration.Generation
	err = r.Status().Update(context.TODO(), migration)
	if err != nil {
		return
	}

	// Done
	return
}

//
// Reflect the plan status.
func (r *Reconciler) reflectPlan(plan *api.Plan, migration *api.Migration) {
	if migration.Status.HasBlockerCondition() {
		return
	}
	if migration.Status.HasAnyCondition(Canceled, Succeeded, Failed) {
		return
	}
	found, snapshot := plan.Status.Migration.SnapshotWithMigration(migration.UID)
	if !found {
		return
	}
	if cnd := snapshot.FindCondition(Canceled); cnd != nil {
		cnd.Durable = true
		migration.Status.SetCondition(*cnd)
		migration.Status.MarkedCompleted()
		return
	}
	if snapshot.HasCondition(Executing) {
		migration.Status.MarkStarted()
		migration.Status.VMs = plan.Status.Migration.VMs
		migration.Status.SetCondition(libcnd.Condition{
			Type:     Running,
			Status:   True,
			Category: Advisory,
			Message:  "The migration is RUNNING.",
		})
	}
	if snapshot.HasCondition(Succeeded) {
		migration.Status.MarkCompleted()
		migration.Status.SetCondition(libcnd.Condition{
			Type:     Succeeded,
			Status:   True,
			Category: Advisory,
			Message:  "The migration has SUCCEEDED.",
			Durable:  true,
		})
	}
	if snapshot.HasCondition(Failed) {
		migration.Status.MarkCompleted()
		migration.Status.SetCondition(libcnd.Condition{
			Type:     Failed,
			Status:   True,
			Category: Advisory,
			Message:  "The migration has FAILED.",
			Durable:  true,
		})
	}
}
