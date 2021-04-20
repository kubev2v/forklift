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

package host

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
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	// Name.
	Name = "host"
)

//
// Package logger.
var log = logging.WithName(Name)

//
// Application settings.
var Settings = &settings.Settings

//
// Creates a new Host Controller and adds it to the Manager.
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
		&source.Kind{Type: &api.Host{}},
		&handler.EnqueueRequestForObject{},
		&HostPredicate{})
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
		libref.Handler(&api.Host{}),
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
// Reconciles a Host object.
type Reconciler struct {
	base.Reconciler
}

//
// Reconcile a Host CR.
// Note: Must not a pointer receiver to ensure that the
// logger and other state is not shared.
func (r Reconciler) Reconcile(request reconcile.Request) (result reconcile.Result, err error) {
	r.Log = logging.WithName(
		names.SimpleNameGenerator.GenerateName(Name+"|"),
		"host",
		request)
	r.Started()
	defer func() {
		result.RequeueAfter = r.Ended(
			result.RequeueAfter,
			err)
		err = nil
	}()

	// Fetch the CR.
	host := &api.Host{}
	err = r.Get(context.TODO(), request.NamespacedName, host)
	if err != nil {
		if k8serr.IsNotFound(err) {
			r.Log.Info("Host deleted.")
			err = nil
		}
		return
	}
	defer func() {
		r.Log.V(1).Info("Conditions.", "all", host.Status.Conditions)
	}()

	// Begin staging conditions.
	host.Status.BeginStagingConditions()

	// Validations.
	err = r.validate(host)
	if err != nil {
		return
	}

	// Ready condition.
	if !host.Status.HasBlockerCondition() && host.Status.HasCondition(ConnectionTestSucceeded) {
		host.Status.SetCondition(libcnd.Condition{
			Type:     libcnd.Ready,
			Status:   True,
			Category: Required,
			Message:  "The host is ready.",
		})
	}

	// End staging conditions.
	host.Status.EndStagingConditions()

	// Record events.
	r.Record(host, host.Status.Conditions)

	// Apply changes.
	host.Status.ObservedGeneration = host.Generation
	err = r.Status().Update(context.TODO(), host)
	if err != nil {
		return
	}

	// ReQ.
	if !host.Status.HasCondition(ConnectionTestSucceeded) {
		result.RequeueAfter = base.SlowReQ
	}

	// Done
	return
}
