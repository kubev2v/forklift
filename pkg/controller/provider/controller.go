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

package provider

import (
	"context"
	libcnd "github.com/konveyor/controller/pkg/condition"
	liberr "github.com/konveyor/controller/pkg/error"
	libcontainer "github.com/konveyor/controller/pkg/inventory/container"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	libweb "github.com/konveyor/controller/pkg/inventory/web"
	"github.com/konveyor/controller/pkg/logging"
	libref "github.com/konveyor/controller/pkg/ref"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/container"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/model"
	ocpmodel "github.com/konveyor/forklift-controller/pkg/controller/provider/model/ocp"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web"
	"github.com/konveyor/forklift-controller/pkg/settings"
	core "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"os"
	"path/filepath"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"time"
)

const (
	// Controller name.
	Name = "provider"
	// Fast re-queue delay.
	FastReQ = time.Millisecond * 500
	// Slow re-queue delay.
	SlowReQ = time.Second * 10
)

//
// Package logger.
var log = logging.WithName(Name)

//
// Application settings.
var Settings = &settings.Settings

func init() {
	container.Log = &log
	web.Log = &log
	model.Log = &log
}

//
// Creates a new Inventory Controller and adds it to the Manager.
func Add(mgr manager.Manager) error {
	restCfg, err := config.GetConfig()
	if err != nil {
		panic(err)
	}
	nClient, err := client.New(
		restCfg,
		client.Options{
			Scheme: scheme.Scheme,
		})
	if err != nil {
		panic(err)
	}
	container := libcontainer.New()
	web := libweb.New(container, web.All(container)...)
	web.Port = Settings.Inventory.Port
	web.TLS.Enabled = Settings.Inventory.TLS.Enabled
	web.TLS.Certificate = Settings.Inventory.TLS.Certificate
	web.TLS.Key = Settings.Inventory.TLS.Key
	web.AllowedOrigins = Settings.CORS.AllowedOrigins
	reconciler := &Reconciler{
		Client:        nClient,
		EventRecorder: mgr.GetEventRecorderFor(Name),
		scheme:        mgr.GetScheme(),
		container:     container,
		web:           web,
	}

	web.Start()

	cnt, err := controller.New(
		Name,
		mgr,
		controller.Options{
			MaxConcurrentReconciles: 10,
			Reconciler:              reconciler,
		})
	if err != nil {
		log.Trace(err)
		return err
	}
	// Primary CR.
	err = cnt.Watch(
		&source.Kind{Type: &api.Provider{}},
		&handler.EnqueueRequestForObject{},
		&ProviderPredicate{})
	if err != nil {
		log.Trace(err)
		return err
	}
	// References.
	err = cnt.Watch(
		&source.Kind{
			Type: &core.Secret{},
		},
		libref.Handler())
	if err != nil {
		log.Trace(err)
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &Reconciler{}

//
// Reconciles an provider object.
type Reconciler struct {
	client.Client
	record.EventRecorder
	scheme    *runtime.Scheme
	container *libcontainer.Container
	web       *libweb.WebServer
}

//
// Reconcile a Inventory CR.
func (r *Reconciler) Reconcile(request reconcile.Request) (result reconcile.Result, err error) {
	fastReQ := reconcile.Result{RequeueAfter: FastReQ}
	slowReQ := reconcile.Result{RequeueAfter: SlowReQ}
	noReQ := reconcile.Result{}
	result = noReQ

	// Reset the logger.
	log.Reset()
	log.SetValues("provider", request)
	log.Info("Reconcile")

	defer func() {
		if err != nil {
			log.Trace(err)
			err = nil
		}
	}()

	// Fetch the CR.
	provider := &api.Provider{}
	err = r.Get(context.TODO(), request.NamespacedName, provider)
	if err != nil {
		if k8serr.IsNotFound(err) {
			err = nil
			deleted := &api.Provider{
				ObjectMeta: meta.ObjectMeta{
					Namespace: request.Namespace,
					Name:      request.Name,
				},
			}
			if r, found := r.container.Delete(deleted); found {
				r.Shutdown()
				r.DB().Close(true)
			}
		} else {
			result = fastReQ
		}
		return
	}
	defer func() {
		log.Info("Conditions.", "all", provider.Status.Conditions)
	}()

	// Updated.
	if !provider.HasReconciled() {
		if r, found := r.container.Delete(provider); found {
			r.Shutdown()
			r.DB().Close(true)
		}
	}

	// Begin staging conditions.
	provider.Status.BeginStagingConditions()

	// Validations.
	err = r.validate(provider)
	if err != nil {
		result = fastReQ
		return
	}

	// Update the container.
	err = r.updateContainer(provider)
	if err != nil {
		result = slowReQ
		return
	}

	// Ready condition.
	if !provider.Status.HasBlockerCondition() &&
		provider.Status.HasCondition(ConnectionTestSucceeded, InventoryCreated) {
		provider.Status.SetCondition(
			libcnd.Condition{
				Type:     libcnd.Ready,
				Status:   True,
				Category: Required,
				Message:  "The provider is ready.",
			})
	}

	// End staging conditions.
	provider.Status.EndStagingConditions()

	// Record events.
	provider.Status.RecordEvents(provider, r)

	// Apply changes.
	provider.Status.ObservedGeneration = provider.Generation
	err = r.Status().Update(context.TODO(), provider)
	if err != nil {
		result = fastReQ
		return
	}

	// Update the DB.
	err = r.updateProvider(provider)
	if err != nil {
		result = fastReQ
		return
	}

	// ReQ.
	if !provider.Status.HasCondition(ConnectionTestSucceeded, InventoryCreated) {
		result = slowReQ
	}

	// Done
	return
}

//
// Update the provider.
func (r *Reconciler) updateProvider(provider *api.Provider) (err error) {
	rl, found := r.container.Get(provider)
	if !found {
		return
	}
	*(rl.Owner().(*api.Provider)) = *provider
	db := rl.DB()
	tx, err := db.Begin()
	if err != nil {
		return
	}
	defer func() {
		_ = tx.End()
	}()
	p := &ocpmodel.Provider{}
	p.With(provider)
	err = tx.Update(p)
	if err == nil {
		err = tx.Commit()
	}

	return
}

//
// Update the container.
func (r *Reconciler) updateContainer(provider *api.Provider) (err error) {
	if _, found := r.container.Get(provider); found {
		if provider.HasReconciled() {
			return
		}
	}
	if provider.Status.HasBlockerCondition() ||
		!provider.Status.HasCondition(ConnectionTestSucceeded) {
		return nil
	}
	db := r.getDB(provider)
	secret, err := r.getSecret(provider)
	if err != nil {
		return err
	}
	err = db.Open(true)
	if err != nil {
		return err
	}
	pModel := &ocpmodel.Provider{}
	pModel.With(provider)
	err = db.Insert(pModel)
	if err != nil {
		return err
	}
	new := container.Build(db, provider, secret)
	current, found, err := r.container.Replace(new)
	if err != nil {
		return err
	}
	if found {
		current.DB().Close(true)
	}

	return nil
}

//
// Build DB for provider.
func (r *Reconciler) getDB(provider *api.Provider) (db libmodel.DB) {
	dir := Settings.Inventory.WorkingDir
	dir = filepath.Join(dir, provider.Namespace)
	os.MkdirAll(dir, 0755)
	file := provider.Name + ".db"
	path := filepath.Join(dir, file)
	models := model.Models(provider)
	db = libmodel.New(path, models...)
	return
}

//
// Get the secret referenced by the provider.
func (r *Reconciler) getSecret(provider *api.Provider) (*core.Secret, error) {
	secret := &core.Secret{}
	if provider.IsHost() {
		return secret, nil
	}
	ref := provider.Spec.Secret
	key := client.ObjectKey{
		Namespace: ref.Namespace,
		Name:      ref.Name,
	}
	err := r.Get(context.TODO(), key, secret)
	if err != nil {
		return nil, liberr.Wrap(err)
	}

	return secret, nil
}
