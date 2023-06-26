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
	"path"
	"sort"
	"time"

	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	planapi "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/plan"
	"github.com/konveyor/forklift-controller/pkg/controller/base"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	libcnd "github.com/konveyor/forklift-controller/pkg/lib/condition"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	"github.com/konveyor/forklift-controller/pkg/lib/logging"
	libref "github.com/konveyor/forklift-controller/pkg/lib/ref"
	"github.com/konveyor/forklift-controller/pkg/settings"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apiserver/pkg/storage/names"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	// Name.
	Name = "plan"
	// Annotation for populator Labels set
	AnnPopulatorLabels = "populatorLabels"
)

// Package logger.
var log = logging.WithName(Name)

// Application settings.
var Settings = &settings.Settings

// Creates a new Plan Controller and adds it to the Manager.
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
		&source.Kind{Type: &api.Plan{}},
		&handler.EnqueueRequestForObject{},
		&PlanPredicate{})
	if err != nil {
		log.Trace(err)
		return err
	}
	//
	// The channel (source) provides a method of queuing
	// events when changes to the provider inventory are detected.
	channel := make(chan event.GenericEvent, 10)
	err = cnt.Watch(
		&source.Channel{Source: channel},
		&handler.EnqueueRequestForObject{})
	if err != nil {
		log.Trace(err)
		return err
	}
	// References.
	// Provider.
	err = cnt.Watch(
		&source.Kind{
			Type: &api.Provider{},
		},
		libref.Handler(&api.Plan{}),
		&ProviderPredicate{
			client:  mgr.GetClient(),
			channel: channel,
		})
	if err != nil {
		log.Trace(err)
		return err
	}
	// NetworkMap.
	err = cnt.Watch(
		&source.Kind{
			Type: &api.NetworkMap{},
		},
		libref.Handler(&api.Plan{}),
		&NetMapPredicate{})
	if err != nil {
		log.Trace(err)
		return err
	}
	// StorageMap.
	err = cnt.Watch(
		&source.Kind{
			Type: &api.StorageMap{},
		},
		libref.Handler(&api.Plan{}),
		&DsMapPredicate{})
	if err != nil {
		log.Trace(err)
		return err
	}
	// Hook..
	err = cnt.Watch(
		&source.Kind{
			Type: &api.Hook{},
		},
		handler.EnqueueRequestsFromMapFunc(RequestForMigration),
		&HookPredicate{})
	if err != nil {
		log.Trace(err)
		return err
	}
	// Migration.
	err = cnt.Watch(
		&source.Kind{
			Type: &api.Migration{},
		},
		handler.EnqueueRequestsFromMapFunc(RequestForMigration),
		&MigrationPredicate{})
	if err != nil {
		log.Trace(err)
		return err
	}

	// Gather migration Plan metrics
	recordMetrics(mgr.GetClient())

	return nil
}

var _ reconcile.Reconciler = &Reconciler{}

// Reconciles a Plan object.
type Reconciler struct {
	base.Reconciler
}

// Reconcile a Plan CR.
// Note: Must not a pointer receiver to ensure that the
// logger and other state is not shared.
func (r Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (result reconcile.Result, err error) {
	r.Log = logging.WithName(
		names.SimpleNameGenerator.GenerateName(Name+"|"),
		"plan",
		request)
	r.Started()
	defer func() {
		result.RequeueAfter = r.Ended(
			result.RequeueAfter,
			err)
		err = nil
	}()

	// Fetch the CR.
	plan := &api.Plan{}
	err = r.Get(context.TODO(), request.NamespacedName, plan)
	if err != nil {
		if k8serr.IsNotFound(err) {
			r.Log.Info("Plan deleted.")
			err = nil
		}
		return
	}
	defer func() {
		r.Log.V(2).Info("Conditions.", "all", plan.Status.Conditions)
	}()

	// Don't reconcile if the plan is archived.
	if plan.Spec.Archived && plan.Status.HasCondition(Archived) {
		r.Log.Info("Aborting reconcile of archived plan.")
		return
	}

	// Postpone as needed.
	postpone, err := r.postpone()
	if err != nil {
		return
	}
	if postpone {
		result.RequeueAfter = base.SlowReQ
		r.Log.Info("Plan Postponed.")
	}

	// Begin staging conditions.
	plan.Status.BeginStagingConditions()

	// Validations.
	err = r.validate(plan)
	if err != nil {
		return
	}

	// Set PopulatorDataSource with labels when needed
	// must be after Referenced data is set (in 'validate')
	if _, ok := plan.Annotations[AnnPopulatorLabels]; !ok {
		r.setPopulatorDataSourceLabels(plan)
	}

	// Archive the plan.
	if plan.Spec.Archived {
		r.archive(plan)
	}

	// Ready condition.
	if !plan.Status.HasBlockerCondition() && !plan.Status.HasCondition(Archived) {
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
	r.Record(plan, plan.Status.Conditions)

	// Apply changes.
	plan.Status.ObservedGeneration = plan.Generation
	// At this point, the plan contains data that is not persisted by design, like the Referenced data
	// and the staged flags in the status, and more data that has been loaded in the validate function,
	// like the name of the VMs in the spec section, therefore we don't want the plan to be overridden
	// by data from the server (even the spec section is overridden) and so we pass a copy of the plan
	err = r.Status().Update(context.TODO(), plan.DeepCopy())
	if err != nil {
		return
	}

	//
	// Execute.
	// The plan is updated as needed to reflect status.
	result.RequeueAfter, err = r.execute(plan)
	if err != nil {
		return
	}

	// Done.
	return
}

func (r *Reconciler) setPopulatorDataSourceLabels(plan *api.Plan) {
	if plan.Status.HasAnyCondition(Succeeded, Failed, Canceled) {
		ctx, err := plancontext.New(r, plan, r.Log)
		if err != nil {
			r.Log.Error(err, "Couldn't construct plan context when trying to set populator labels.")
		} else {
			runner := Migration{Context: ctx}
			runner.SetPopulatorDataSourceLabels()
			planCopy := plan.DeepCopy()
			if plan.Annotations == nil {
				plan.Annotations = make(map[string]string)
			}
			plan.Annotations[AnnPopulatorLabels] = "True"
			patch := client.MergeFrom(planCopy)
			r.Client.Patch(context.TODO(), plan, patch)
			// Restore Referenced data that is not returned back from the server
			plan.Referenced = planCopy.Referenced
			// Restore original status with staged conditions
			plan.Status = planCopy.Status
			// Restore original spec with VM names
			plan.Spec = planCopy.Spec
		}
	}
}

// Archive the plan.
// Makes a best-effort attempt to clean up lingering
// plan resources.
func (r *Reconciler) archive(plan *api.Plan) {
	ctx, err := plancontext.New(r, plan, r.Log)
	if err != nil {
		r.Log.Error(err, "Couldn't construct plan context while archiving plan.")
	} else {
		runner := Migration{Context: ctx}
		runner.Archive()
	}
	// Regardless of whether or not we can clean up, mark the plan archived.
	plan.Status.SetCondition(
		libcnd.Condition{
			Type:     Archived,
			Status:   True,
			Category: Advisory,
			Reason:   UserRequested,
			Message:  "The migration plan has been archived.",
		})
	r.Log.Info("Plan archived.")
}

// Execute the plan.
//  1. Find active (current) migration.
//  2. If found, update the context and match the snapshot.
//  3. Cancel as needed.
//  4. If not, find the next pending migration.
//  5. If a new migration is being started, update the context and snapshot.
//  6. Run the migration.
func (r *Reconciler) execute(plan *api.Plan) (reQ time.Duration, err error) {
	if plan.Status.HasBlockerCondition() || plan.Status.HasCondition(Archived) {
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
	ctx, err := plancontext.New(r, plan, r.Log)
	if err != nil {
		return
	}
	//
	// Find and validate the current (active) migration.
	migration, err := r.activeMigration(plan)
	if err != nil {
		return
	}
	if migration != nil {
		ctx.SetMigration(migration)
		r.matchSnapshot(ctx)
		r.Log.Info(
			"Found (active) migration.",
			"migration",
			path.Join(
				migration.GetNamespace(),
				migration.GetName()))
	}
	//
	// The active snapshot may be marked canceled by:
	//   activeMigration()
	//   matchSnapshot()
	snapshot := plan.Status.Migration.ActiveSnapshot()
	if snapshot.HasCondition(Canceled) {
		r.Log.Info("migration (active) marked as canceled.")
		for _, vm := range plan.Status.Migration.VMs {
			if !vm.HasAnyCondition(Succeeded, Failed) {
				vm.SetCondition(
					libcnd.Condition{
						Type:     Canceled,
						Status:   True,
						Category: Advisory,
						Reason:   Modified,
						Message:  "The migration has been canceled.",
						Durable:  true,
					})
				r.Log.Info(
					"Snapshot canceled condition copied to VM.",
					"vm",
					vm.String())
			}
		}
	}
	//
	// Cancel.
	runner := Migration{Context: ctx}
	err = runner.Cancel()
	if err != nil {
		return
	}
	//
	// Find pending migrations.
	pending := []*api.Migration{}
	pending, err = r.pendingMigrations(plan)
	if err != nil {
		return
	}
	//
	// No active migration.
	// Select the next pending migration as the (active) migration.
	if migration == nil && len(pending) > 0 {
		migration = pending[0]
		ctx.SetMigration(migration)
		snapshot = r.newSnapshot(ctx)
		plan.Status.DeleteCondition(Failed, Canceled)
		r.Log.Info(
			"Found (new) migration.",
			"migration",
			path.Join(
				migration.GetNamespace(),
				migration.GetName()))
	}
	//
	// No (active) migration.
	// Done.
	if migration == nil {
		r.Log.Info("No pending migrations found.")
		plan.Status.DeleteCondition(Executing)
		reQ = NoReQ
		return
	}

	//
	// Run the migration.
	snapshot.BeginStagingConditions()
	runner = Migration{Context: ctx}
	reQ, err = runner.Run()
	if err != nil {
		return
	}
	snapshot.EndStagingConditions()

	// Reflect the active snapshot status on the plan.
	for _, t := range []string{Executing, Succeeded, Failed, Canceled} {
		if cnd := snapshot.FindCondition(t); cnd != nil {
			r.Log.V(2).Info(
				"Snapshot condition copied to plan.",
				"condition",
				cnd)
			plan.Status.SetCondition(*cnd)
		} else {
			plan.Status.DeleteCondition(t)
			r.Log.V(2).Info(
				"Snapshot condition cleared on plan.",
				"condition",
				t)
		}
	}
	if len(pending) > 1 && reQ == 0 {
		r.Log.V(1).Info(
			"Found pending migrations.",
			"count",
			len(pending))
		reQ = base.FastReQ
	}

	return
}

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
	snapshot.Map.Network.With(plan.Referenced.Map.Network)
	snapshot.Map.Storage.With(plan.Referenced.Map.Storage)
	plan.Status.Migration.NewSnapshot(snapshot)
	log.V(1).Info(
		"Snapshot created.",
		"snapshot",
		snapshot)
	return plan.Status.Migration.ActiveSnapshot()
}

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
		log.Info("Snapshot: plan not matched.")
		return false
	}
	if !snapshot.Provider.Source.Match(plan.Referenced.Provider.Source) {
		log.Info("Snapshot: provider (source) not matched.")
		return false
	}
	if !snapshot.Provider.Destination.Match(plan.Referenced.Provider.Destination) {
		log.Info("Snapshot: provider (destination) not matched.")
		return false
	}
	if !snapshot.Map.Network.Match(plan.Referenced.Map.Network) {
		log.Info("Snapshot: networkMap not matched.")
		return false
	}
	if !snapshot.Map.Storage.Match(plan.Referenced.Map.Storage) {
		log.Info("Snapshot: storageMap not matched.")
		return false
	}

	return true
}

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
	// the migration is inactive if it's reached a terminal state
	if snapshot.HasAnyCondition(Canceled, Failed, Succeeded) {
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
			r.Log.Info("Active snapshot deleted.")
			err = nil
		} else {
			err = liberr.Wrap(err)
		}
		return
	}
	if active.UID != snapshot.Migration.UID {
		r.Log.Info("Active snapshot deleted.")
		snapshot.SetCondition(deleted)
		active = nil
	}

	migration = active

	return
}

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
				r.Log.Info(
					"Migration ignored.",
					"migration",
					path.Join(
						migration.GetNamespace(),
						migration.GetName()))
				continue
			}
		}
		if migration.Status.HasAnyCondition(Succeeded, Failed, Canceled) {
			r.Log.Info(
				"Migration ignored.",
				"migration",
				path.Join(
					migration.GetNamespace(),
					migration.GetName()))
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

// Postpone reconciliation.
// Ensure that dependencies (CRs) have been reconciled.
func (r *Reconciler) postpone() (postpone bool, err error) {
	// Provider.
	providerList := &api.ProviderList{}
	err = r.List(context.TODO(), providerList)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	for _, provider := range providerList.Items {
		if provider.Status.ObservedGeneration < provider.Generation {
			postpone = true
			r.Log.V(1).Info(
				"Postponing: provider not reconciled.",
				"provider",
				path.Join(
					provider.GetNamespace(),
					provider.GetName()))
			return
		}
	}
	// NetworkMap
	netMapList := &api.NetworkMapList{}
	err = r.List(context.TODO(), netMapList)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	for _, netMap := range netMapList.Items {
		if netMap.Status.ObservedGeneration < netMap.Generation {
			postpone = true
			r.Log.V(1).Info(
				"Postponing: networkMap not reconciled.",
				"map",
				path.Join(
					netMap.GetNamespace(),
					netMap.GetName()))
			return
		}
	}
	// StorageMap
	dsMapList := &api.StorageMapList{}
	err = r.List(context.TODO(), dsMapList)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	for _, dsMap := range netMapList.Items {
		if dsMap.Status.ObservedGeneration < dsMap.Generation {
			postpone = true
			r.Log.V(1).Info(
				"Postponing: storageMap not reconciled.",
				"map",
				path.Join(
					dsMap.GetNamespace(),
					dsMap.GetName()))
			return
		}
	}
	// Host
	hostList := &api.HostList{}
	err = r.List(context.TODO(), hostList)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	for _, host := range hostList.Items {
		if host.Status.ObservedGeneration < host.Generation {
			postpone = true
			r.Log.V(1).Info(
				"Postponing: host not reconciled.",
				"host",
				path.Join(
					host.GetNamespace(),
					host.GetName()))
			return
		}
	}
	// Hook.
	hookList := &api.HookList{}
	err = r.List(context.TODO(), hookList)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	for _, hook := range hookList.Items {
		if hook.Status.ObservedGeneration < hook.Generation {
			postpone = true
			r.Log.V(1).Info(
				"Postponing: hook not reconciled.",
				"hook",
				path.Join(
					hook.GetNamespace(),
					hook.GetName()))
			return
		}
	}

	return
}
