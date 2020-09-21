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

package network

import (
	"context"
	"errors"
	cnd "github.com/konveyor/controller/pkg/condition"
	"github.com/konveyor/controller/pkg/logging"
	libref "github.com/konveyor/controller/pkg/ref"
	api "github.com/konveyor/virt-controller/pkg/apis/virt/v1alpha1"
	"github.com/konveyor/virt-controller/pkg/settings"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
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
	Name = "network-map"
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
// Creates a new Map Controller and adds it to the Manager.
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
		&source.Kind{Type: &api.NetworkMap{}},
		&handler.EnqueueRequestForObject{},
		&MapPredicate{})
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

	return nil
}

var _ reconcile.Reconciler = &Reconciler{}

//
// Reconciles a Map object.
type Reconciler struct {
	client.Client
	scheme *runtime.Scheme
}

//
// Reconcile a Map CR.
func (r *Reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	fastReQ := reconcile.Result{RequeueAfter: FastReQ}
	noReQ := reconcile.Result{}
	var err error

	// Reset the logger.
	log.Reset()
	log.SetValues("map", request.Name)

	// Fetch the CR.
	mp := &api.NetworkMap{}
	err = r.Get(context.TODO(), request.NamespacedName, mp)
	if err != nil {
		if k8serr.IsNotFound(err) {
			return noReQ, nil
		}
		log.Trace(err)
		return noReQ, err
	}

	// Begin staging conditions.
	mp.Status.BeginStagingConditions()

	// Validations.
	err = r.validate(mp)
	if err != nil {
		if errors.Is(err, ProviderInvNotReady) {
			return fastReQ, nil
		}
		log.Trace(err)
		return fastReQ, nil
	}

	// Ready condition.
	if !mp.Status.HasBlockerCondition() {
		mp.Status.SetCondition(cnd.Condition{
			Type:     cnd.Ready,
			Status:   True,
			Category: Required,
			Message:  "The network map is ready.",
		})
	}

	// End staging conditions.
	mp.Status.EndStagingConditions()

	// Apply changes.
	mp.Status.ObservedGeneration = mp.Generation
	err = r.Status().Update(context.TODO(), mp)
	if err != nil {
		log.Trace(err)
		return fastReQ, nil
	}

	// Done
	return noReQ, nil
}
