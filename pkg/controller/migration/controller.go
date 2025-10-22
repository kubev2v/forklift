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

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/base"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	libref "github.com/kubev2v/forklift/pkg/lib/ref"
	metrics "github.com/kubev2v/forklift/pkg/monitoring/metrics/forklift-controller"
	"github.com/kubev2v/forklift/pkg/settings"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apiserver/pkg/storage/names"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	k8sutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	// Name.
	Name = "migration"
)

// Package logger.
var log = logging.WithName(Name)

// Application settings.
var Settings = &settings.Settings

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
		source.Kind(mgr.GetCache(), &api.Migration{},
			&handler.TypedEnqueueRequestForObject[*api.Migration]{},
			&MigrationPredicate{}))
	if err != nil {
		log.Trace(err)
		return err
	}
	// References.
	err = cnt.Watch(
		source.Kind(mgr.GetCache(), &api.Plan{},
			libref.TypedHandler[*api.Plan](&api.Migration{}),
			&PlanPredicate{}))
	if err != nil {
		log.Trace(err)
		return err
	}

	// Gather migration metrics
	metrics.RecordMigrationMetrics(mgr.GetClient())

	return nil
}

var _ reconcile.Reconciler = &Reconciler{}

// Reconciles a Migration object.
type Reconciler struct {
	base.Reconciler
}

// Reconcile a Migration CR.
// Note: Must not a pointer receiver to ensure that the
// logger and other state is not shared.
func (r Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (result reconcile.Result, err error) {
	r.Log = logging.WithName(
		names.SimpleNameGenerator.GenerateName(Name+"|"),
		"migration",
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
		r.Log.V(2).Info("Conditions.", "all", migration.Status.Conditions)
	}()

	// Set owner reference for migration CR if it was created using CLI
	// Try to set owner reference before doing anything with the migration CR so we fail fast.
	// Skip setting of owner reference if plan is not found
	err = r.setOwnerReference(migration)
	if err != nil {
		r.Log.Error(err, "Could not set migration owner reference.")
		return
	}

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
	migration.Status.VMs = plan.Status.Migration.VMs
}

// Set owner reference for Migration.
// This is needed so the migration CR will be auto deleted once the plan CR is deleted.
//
// Update owner reference to an owning plan or return error if any occured.
//
// Arguments:
//   - migration (*api.Migration): Migration object to which owner reference will be set
//
// Returns:
//   - error: An error if something goes wrong during the process.
func (r *Reconciler) setOwnerReference(migration *api.Migration) error {
	plan := &api.Plan{}
	err := r.Client.Get(
		context.TODO(),
		client.ObjectKey{
			Namespace: migration.Spec.Plan.Namespace,
			Name:      migration.Spec.Plan.Name,
		},
		plan,
	)
	if err != nil {
		// Ignore setting of owner ref if the plan is was not found e.g. deleted
		if k8serr.IsNotFound(err) {
			err = nil
		}
		return err
	}

	err = k8sutil.SetOwnerReference(plan, migration, r.Scheme())
	if err != nil {
		return err
	}

	err = r.Client.Update(context.TODO(), migration)
	if err != nil {
		return err
	}

	err = r.Client.Get(
		context.TODO(),
		client.ObjectKey{
			Namespace: migration.Namespace,
			Name:      migration.Name,
		},
		migration,
	)
	if err != nil {
		return err
	}

	return nil
}
