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
	Name = "host"
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
// Creates a new Host Controller and adds it to the Manager.
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
		&source.Kind{Type: &api.Host{}},
		&handler.EnqueueRequestForObject{},
		&HostPredicate{})
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
// Reconciles a Host object.
type Reconciler struct {
	client.Client
	scheme *runtime.Scheme
}

//
// Reconcile a Host CR.
func (r *Reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	fastReQ := reconcile.Result{RequeueAfter: FastReQ}
	noReQ := reconcile.Result{}
	var err error

	// Reset the logger.
	log.Reset()
	log.SetValues("host", request.Name)

	// Fetch the CR.
	host := &api.Host{}
	err = r.Get(context.TODO(), request.NamespacedName, host)
	if err != nil {
		if errors.IsNotFound(err) {
			return noReQ, nil
		}
		log.Trace(err)
		return noReQ, err
	}

	// Begin staging conditions.
	host.Status.BeginStagingConditions()

	// Validations.
	err = r.validate(host)
	if err != nil {
		log.Trace(err)
		return fastReQ, nil
	}

	// Ready condition.
	if !host.Status.HasBlockerCondition() {
		host.Status.SetCondition(cnd.Condition{
			Type:     cnd.Ready,
			Status:   True,
			Category: Required,
			Message:  "The host is ready.",
		})
	}

	// End staging conditions.
	host.Status.EndStagingConditions()

	// Apply changes.
	host.Status.ObservedGeneration = host.Generation
	err = r.Status().Update(context.TODO(), host)
	if err != nil {
		log.Trace(err)
		return fastReQ, nil
	}

	// Done
	return noReQ, nil
}
