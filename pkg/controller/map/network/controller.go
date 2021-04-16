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
	libcnd "github.com/konveyor/controller/pkg/condition"
	"github.com/konveyor/controller/pkg/logging"
	libref "github.com/konveyor/controller/pkg/ref"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web"
	"github.com/konveyor/forklift-controller/pkg/settings"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
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
// Creates a new Map Controller and adds it to the Manager.
func Add(mgr manager.Manager) error {
	reconciler := &Reconciler{
		EventRecorder: mgr.GetEventRecorderFor(Name),
		Client:        mgr.GetClient(),
		scheme:        mgr.GetScheme(),
		log:           log,
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
	err = cnt.Watch(
		&source.Kind{
			Type: &api.Provider{},
		},
		libref.Handler(&api.NetworkMap{}),
		&ProviderPredicate{
			client:  mgr.GetClient(),
			channel: channel,
		})
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
	record.EventRecorder
	client.Client
	scheme *runtime.Scheme
	log    *logging.Logger
}

//
// Reconcile a Map CR.
// Note: Must not a pointer receiver to ensure that the
// logger and other state is not shared.
func (r Reconciler) Reconcile(request reconcile.Request) (result reconcile.Result, err error) {
	fastReQ := reconcile.Result{RequeueAfter: FastReQ}
	slowReQ := reconcile.Result{RequeueAfter: SlowReQ}
	noReQ := reconcile.Result{}
	result = noReQ

	r.log = logging.WithName(
		names.SimpleNameGenerator.GenerateName(Name+"|"),
		"map",
		request)

	r.log.Info("Reconcile")

	defer func() {
		if err != nil {
			if k8serr.IsConflict(err) {
				r.log.Info(err.Error())
			} else {
				r.log.Trace(err)
			}
			err = nil
		}
	}()

	// Fetch the CR.
	mp := &api.NetworkMap{}
	err = r.Get(context.TODO(), request.NamespacedName, mp)
	if err != nil {
		if k8serr.IsNotFound(err) {
			err = nil
		}
		return
	}
	defer func() {
		r.log.Info("Conditions.", "all", mp.Status.Conditions)
	}()

	// Begin staging conditions.
	mp.Status.BeginStagingConditions()

	// Validations.
	err = r.validate(mp)
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
	if !mp.Status.HasBlockerCondition() {
		mp.Status.SetCondition(libcnd.Condition{
			Type:     libcnd.Ready,
			Status:   True,
			Category: Required,
			Message:  "The network map is ready.",
		})
	}

	// End staging conditions.
	mp.Status.EndStagingConditions()

	// Record events.
	mp.Status.RecordEvents(mp, r)

	// Apply changes.
	mp.Status.ObservedGeneration = mp.Generation
	err = r.Status().Update(context.TODO(), mp)
	if err != nil {
		result = fastReQ
		return
	}

	// Done
	return
}
