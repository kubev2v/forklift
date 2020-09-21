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
	cnd "github.com/konveyor/controller/pkg/condition"
	"github.com/konveyor/controller/pkg/logging"
	libref "github.com/konveyor/controller/pkg/ref"
	api "github.com/konveyor/virt-controller/pkg/apis/virt/v1alpha1"
	"github.com/konveyor/virt-controller/pkg/settings"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"time"
)

const (
	// Controller name.
	Name = "migration"
	// Fast re-queue delay.
	FastReQ = time.Millisecond * 100
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
		Client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
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
		libref.Handler(),
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
	client.Client
	scheme *runtime.Scheme
}

//
// Reconcile a Migration CR.
func (r *Reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	fastReQ := reconcile.Result{RequeueAfter: FastReQ}
	noReQ := reconcile.Result{}
	reQ := time.Duration(0)
	var err error

	// Reset the logger.
	log.Reset()
	log.SetValues("migration", request.Name)

	// Fetch the CR.
	migration := &api.Migration{}
	err = r.Get(context.TODO(), request.NamespacedName, migration)
	if err != nil {
		if errors.IsNotFound(err) {
			return noReQ, nil
		}
		log.Trace(err)
		return noReQ, err
	}

	// Begin staging conditions.
	migration.Status.BeginStagingConditions()

	// Validations.
	err = r.validate(migration)
	if err != nil {
		log.Trace(err)
		return fastReQ, nil
	}

	// Ready condition.
	if !migration.Status.HasBlockerCondition() {
		migration.Status.SetCondition(cnd.Condition{
			Type:     cnd.Ready,
			Status:   True,
			Category: Required,
			Message:  "The migration is ready.",
		})
	}

	// Run migration.
	task := Task{
		Client:    r,
		Migration: migration,
	}
	reQ, err = task.Run()
	if err != nil {
		log.Trace(err)
		return fastReQ, nil
	}

	// End staging conditions.
	migration.Status.EndStagingConditions()

	// Apply changes.
	migration.Status.ObservedGeneration = migration.Generation
	err = r.Status().Update(context.TODO(), migration)
	if err != nil {
		log.Trace(err)
		return reconcile.Result{Requeue: true}, nil
	}

	if reQ > NoReQ {
		return reconcile.Result{RequeueAfter: reQ}, nil
	}

	return noReQ, nil
}
