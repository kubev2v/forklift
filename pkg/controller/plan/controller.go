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
	"errors"
	libcnd "github.com/konveyor/controller/pkg/condition"
	liberr "github.com/konveyor/controller/pkg/error"
	"github.com/konveyor/controller/pkg/logging"
	libref "github.com/konveyor/controller/pkg/ref"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	planapi "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1/plan"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web"
	"github.com/konveyor/forklift-controller/pkg/settings"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"path"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"sort"
	"time"
)

const (
	// Controller name.
	Name = "plan"
	// Fast re-queue delay.
	FastReQ = time.Millisecond * 500
	// Slow re-queue delay.
	SlowReQ = time.Second * 3
)

//
// Package logger.
var log = logging.WithName(Name)

//
// Application settings.
var Settings = &settings.Settings

//
// Creates a new Plan Controller and adds it to the Manager.
func Add(mgr manager.Manager) error {
	reconciler := &Reconciler{
		EventRecorder: mgr.GetEventRecorderFor(Name),
		Client:        mgr.GetClient(),
		scheme:        mgr.GetScheme(),
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
		&source.Kind{Type: &api.Plan{}},
		&handler.EnqueueRequestForObject{},
		&PlanPredicate{})
	if err != nil {
		log.Trace(err)
		return err
	}
	// References.
	err = cnt.Watch(
		&source.Kind{
			Type: &api.Provider{},
		},
		libref.Handler(),
		&ProviderPredicate{})
	if err != nil {
		log.Trace(err)
		return err
	}
	err = cnt.Watch(
		&source.Kind{
			Type: &api.Migration{},
		},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: handler.ToRequestsFunc(RequestForMigration),
		},
		&MigrationPredicate{})
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
	record.EventRecorder
	client.Client
	scheme *runtime.Scheme
}

//
// Reconcile a Plan CR.
func (r *Reconciler) Reconcile(request reconcile.Request) (result reconcile.Result, err error) {
	fastReQ := reconcile.Result{RequeueAfter: FastReQ}
	slowReQ := reconcile.Result{RequeueAfter: SlowReQ}
	noReQ := reconcile.Result{}
	result = noReQ

	// Reset the logger.
	log.Reset()
	log.SetValues("plan", request)
	log.Info("Reconcile", "plan", request)

	defer func() {
		if err != nil {
			log.Trace(err)
			err = nil
		}
	}()

	// Fetch the CR.
	plan := &api.Plan{}
	err = r.Get(context.TODO(), request.NamespacedName, plan)
	if err != nil {
		if k8serr.IsNotFound(err) {
			err = nil
		}
		return
	}
	defer func() {
		log.Info("Conditions.", "all", plan.Status.Conditions)
	}()

	// Postpone as needed.
	postpone, err := r.postpone()
	if err != nil {
		log.Trace(err)
		return slowReQ, err
	}
	if postpone {
		log.Info("Postponed")
		return slowReQ, nil
	}

	// Begin staging conditions.
	plan.Status.BeginStagingConditions()

	// Validations.
	err = r.validate(plan)
	if err != nil {
		if errors.As(err, &web.ProviderNotReadyError{}) {
			result = slowReQ
			err = nil
		} else {
			result = fastReQ
		}
		return
	}

	// Ready condition.
	if !plan.Status.HasBlockerCondition() {
		plan.Status.SetCondition(libcnd.Condition{
			Type:     libcnd.Ready,
			Status:   True,
			Category: Required,
			Message:  "The migration plan is ready.",
		})
	}

	// End staging conditions.
	plan.Status.EndStagingConditions()

	// Record events.
	plan.Status.RecordEvents(plan, r)

	// Apply changes.
	plan.Status.ObservedGeneration = plan.Generation
	err = r.Status().Update(context.TODO(), plan)
	if err != nil {
		result = fastReQ
		return
	}

	//
	// Execute.
	// The plan is updated as needed to reflect status.
	reQ, err := r.execute(plan)
	if err != nil {
		result = fastReQ
		return
	}

	// Done
	if reQ > 0 {
		result = reconcile.Result{RequeueAfter: reQ}
	} else {
		result = noReQ
	}

	return
}

//
// Execute the plan.
//   1. Find active (current) migration.
//   2. If found, update the context and match the snapshot.
//   3. Cancel as needed.
//   4. If not, find the next pending migration.
//   5. If a new migration is being started, update the context and snapshot.
//   6. Run the migration.
func (r *Reconciler) execute(plan *api.Plan) (reQ time.Duration, err error) {
	if plan.Status.HasBlockerCondition() {
		return
	}
	defer func() {
		if err == nil {
			err = r.Status().Update(context.TODO(), plan)
			if err != nil {
				err = liberr.Wrap(err)
			}
		}
	}()
	var migration *api.Migration
	snapshot := plan.Status.Migration.ActiveSnapshot()
	ctx, err := plancontext.New(
		r,
		plan,
		&api.Migration{
			ObjectMeta: meta.ObjectMeta{
				Namespace: snapshot.Migration.Namespace,
				Name:      snapshot.Migration.Name,
				UID:       snapshot.Migration.UID,
			},
		})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	//
	// Find and validate the current (active) migration.
	migration, err = r.activeMigration(plan)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	if migration != nil {
		ctx.Migration = migration
		r.matchSnapshot(ctx)
	}
	//
	// The active snapshot may be marked canceled by:
	//   activeMigration()
	//   matchSnapshot()
	if snapshot.HasCondition(Canceled) {
		migration = nil
		runner := Migration{Context: ctx}
		err = runner.Cancel()
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
	}
	//
	// Find pending migrations.
	pending := []*api.Migration{}
	pending, err = r.pendingMigrations(plan)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	//
	// No active migration.
	// Select the next pending migration as the (active) migration.
	if migration == nil && len(pending) > 0 {
		migration = pending[0]
		ctx.Migration = migration
		snapshot = r.newSnapshot(ctx)
	}
	//
	// No (active) migration.
	// Done.
	if migration == nil {
		plan.Status.DeleteCondition(Executing)
		reQ = NoReQ
		return
	}
	//
	// Run the migration.
	snapshot.BeginStagingConditions()
	runner := Migration{Context: ctx}
	reQ, err = runner.Run()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	//
	// Reflect the plan status on the active
	// snapshot in the history.
	for _, t := range []string{Executing, Succeeded, Failed} {
		if cnd := plan.Status.FindCondition(t); cnd != nil {
			snapshot.SetCondition(*cnd)
		}
	}
	snapshot.EndStagingConditions()
	if len(pending) > 1 && reQ == 0 {
		reQ = FastReQ
	}

	return
}

//
// Create a new snapshot.
// Return: The new active snapshot.
func (r *Reconciler) newSnapshot(ctx *plancontext.Context) *planapi.Snapshot {
	plan := ctx.Plan
	migration := ctx.Migration
	snapshot := planapi.Snapshot{}
	snapshot.Plan.With(plan)
	snapshot.Migration.With(migration)
	snapshot.Provider.Source.With(plan.Referenced.Provider.Source)
	snapshot.Provider.Destination.With(plan.Referenced.Provider.Destination)
	plan.Status.Migration.NewSnapshot(snapshot)
	return plan.Status.Migration.ActiveSnapshot()
}

//
// Match the snapshot and detect mutation.
// When detected, the (active) snapshot will get marked as canceled.
func (r *Reconciler) matchSnapshot(ctx *plancontext.Context) (matched bool) {
	plan := ctx.Plan
	snapshot := plan.Status.Migration.ActiveSnapshot()
	defer func() {
		if !matched {
			plan := ctx.Plan
			plan.Status.DeleteCondition(Executing)
			plan.Status.Migration.MarkCompleted()
			snapshot.DeleteCondition(Executing)
			snapshot.SetCondition(
				libcnd.Condition{
					Type:     Canceled,
					Status:   True,
					Category: Advisory,
					Reason:   Modified,
					Message:  "The migration has been canceled.",
					Durable:  true,
				})
		}
	}()
	if !snapshot.Plan.Match(plan) {
		return false
	}
	if !snapshot.Provider.Source.Match(plan.Referenced.Provider.Source) {
		return false
	}
	if !snapshot.Provider.Destination.Match(plan.Referenced.Provider.Destination) {
		return false
	}

	return true
}

//
// Get the current (active) migration referenced in the snapshot.
// The active snapshot will be marked canceled.
// Returns: nil when not-found.
func (r *Reconciler) activeMigration(plan *api.Plan) (migration *api.Migration, err error) {
	snapshot := plan.Status.Migration.ActiveSnapshot()
	defer func() {
		if migration == nil {
			snapshot.DeleteCondition(Executing)
		}
	}()
	if snapshot.HasCondition(Canceled) {
		return
	}
	deleted := libcnd.Condition{
		Type:     Canceled,
		Status:   True,
		Category: Advisory,
		Reason:   Deleted,
		Message:  "The migration has been deleted.",
		Durable:  true,
	}
	active := &api.Migration{}
	err = r.Get(
		context.TODO(),
		client.ObjectKey{
			Namespace: snapshot.Migration.Namespace,
			Name:      snapshot.Migration.Name,
		},
		active)
	if err != nil {
		if k8serr.IsNotFound(err) {
			snapshot.SetCondition(deleted)
			err = nil
		} else {
			err = liberr.Wrap(err)
		}
		return
	}
	if active.UID != snapshot.Migration.UID {
		snapshot.SetCondition(deleted)
		active = nil
	}

	migration = active

	return
}

//
// Sorted list of pending migrations.
func (r *Reconciler) pendingMigrations(plan *api.Plan) (list []*api.Migration, err error) {
	all := &api.MigrationList{}
	err = r.List(context.TODO(), all)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	list = []*api.Migration{}
	for i := range all.Items {
		migration := &all.Items[i]
		if !migration.Match(plan) {
			continue
		}
		if found, snapshot := plan.Status.Migration.SnapshotWithMigration(migration.UID); found {
			if snapshot.HasCondition(Canceled) {
				continue
			}
		}
		if migration.Status.HasAnyCondition(Succeeded, Failed, Canceled) {
			continue
		}
		list = append(list, migration)
	}
	sort.Slice(
		list,
		func(i, j int) bool {
			mA := list[i].ObjectMeta
			mB := list[j].ObjectMeta
			tA := mA.CreationTimestamp
			tB := mB.CreationTimestamp
			if !tA.Equal(&tB) {
				return tA.Before(&tB)
			}
			nA := path.Join(mA.Namespace, mA.Name)
			nB := path.Join(mB.Namespace, mB.Name)
			return nA < nB
		})

	return
}

//
// Postpone reconciliation.
// Ensure that dependencies (CRs) have been reconciled.
func (r *Reconciler) postpone() (postpone bool, err error) {
	providerList := &api.ProviderList{}
	err = r.List(context.TODO(), providerList)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	for _, provider := range providerList.Items {
		if provider.Status.ObservedGeneration < provider.Generation {
			postpone = true
			return
		}
	}
	hostList := &api.HostList{}
	err = r.List(context.TODO(), hostList)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	for _, host := range hostList.Items {
		if host.Status.ObservedGeneration < host.Generation {
			postpone = true
			return
		}
	}

	return
}
