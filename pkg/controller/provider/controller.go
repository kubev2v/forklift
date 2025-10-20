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
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/base"
	"github.com/kubev2v/forklift/pkg/controller/provider/container"
	"github.com/kubev2v/forklift/pkg/controller/provider/model"
	"github.com/kubev2v/forklift/pkg/controller/provider/web"
	"github.com/kubev2v/forklift/pkg/controller/validation/policy"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	libfb "github.com/kubev2v/forklift/pkg/lib/filebacked"
	libcontainer "github.com/kubev2v/forklift/pkg/lib/inventory/container"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
	libweb "github.com/kubev2v/forklift/pkg/lib/inventory/web"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	libref "github.com/kubev2v/forklift/pkg/lib/ref"
	"github.com/kubev2v/forklift/pkg/lib/sshkeys"
	"github.com/kubev2v/forklift/pkg/settings"
	"golang.org/x/crypto/ssh"
	v1 "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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
	Name               = "provider"
	OvaTimeout         = 10 * time.Minute
	OvaReconcilerRetry = 5 * time.Second
)

// Package logger.
var log = logging.WithName(Name)

// Application settings.
var Settings = &settings.Settings

// Creates a new Inventory Controller and adds it to the Manager.
func Add(mgr manager.Manager) error {
	libfb.WorkingDir = Settings.WorkingDir
	container := libcontainer.New()
	web := libweb.New(container, web.All(container)...)
	web.Port = Settings.Inventory.Port
	if Settings.Inventory.TLS.Key != "" {
		web.TLS.Enabled = true
		web.TLS.Certificate = Settings.Inventory.TLS.Certificate
		web.TLS.Key = Settings.Inventory.TLS.Key
		web.AllowedOrigins = Settings.Inventory.CORS.AllowedOrigins
	}
	reconciler := &Reconciler{
		Reconciler: base.Reconciler{
			EventRecorder: mgr.GetEventRecorderFor(Name),
			Client:        mgr.GetClient(),
			Log:           log,
		},
		catalog:   &Catalog{},
		container: container,
		web:       web,
	}

	web.Start()

	policy.Agent.Start()

	cnt, err := controller.New(
		Name,
		mgr,
		controller.Options{
			MaxConcurrentReconciles: Settings.MaxConcurrentReconciles,
			Reconciler:              reconciler,
		})
	if err != nil {
		log.Trace(err)
		return err
	}
	// Primary CR.
	err = cnt.Watch(
		source.Kind(mgr.GetCache(), &api.Provider{},
			&handler.TypedEnqueueRequestForObject[*api.Provider]{},
			&ProviderPredicate{}))
	if err != nil {
		log.Trace(err)
		return err
	}
	// References.
	err = cnt.Watch(
		source.Kind(mgr.GetCache(), &v1.Secret{},
			libref.TypedHandler[*v1.Secret](&api.Provider{})))
	if err != nil {
		log.Trace(err)
		return err
	}
	err = cnt.Watch(
		source.Kind(mgr.GetCache(), &api.OVAProviderServer{},
			libref.TypedHandler[*api.OVAProviderServer](&api.Provider{})))
	if err != nil {
		log.Trace(err)
		return err
	}
	return nil
}

var _ reconcile.Reconciler = &Reconciler{}

// Reconciles an provider object.
type Reconciler struct {
	base.Reconciler
	catalog   *Catalog
	container *libcontainer.Container
	web       *libweb.WebServer
}

// Reconcile a Inventory CR.
// Note: Must not a pointer receiver to ensure that the
// logger and other state is not shared.
func (r Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (result reconcile.Result, err error) {
	r.Log = logging.WithName(
		names.SimpleNameGenerator.GenerateName(Name+"|"),
		"provider",
		request)
	r.Started()
	defer func() {
		result.RequeueAfter = r.Ended(
			result.RequeueAfter,
			err)
		err = nil
	}()

	// Fetch the CR.
	provider := &api.Provider{}
	err = r.Get(context.TODO(), request.NamespacedName, provider)
	if err != nil {
		if k8serr.IsNotFound(err) {
			r.Log.Info("Provider deleted.")
			err = nil
			if deleted, found := r.catalog.get(request); found {
				if r, found := r.container.Delete(deleted); found {
					r.Shutdown()
					_ = r.DB().Close(true)
				}
			}
		}
		return
	} else {
		r.catalog.add(request, provider)

	}

	defer func() {
		r.Log.V(2).Info("Conditions.", "all", provider.Status.Conditions)
	}()

	defer func() {
		// Stop reconciliation when auth fails
		if provider.Status.HasCondition(ConnectionAuthFailed) {
			result.RequeueAfter = 0
			err = nil
			return
		}
	}()

	// Updated.
	if !provider.HasReconciled() {
		if r, found := r.container.Delete(provider); found {
			r.Shutdown()
			_ = r.DB().Close(true)
		}
	}

	// Begin staging conditions.
	provider.Status.Phase = Staging
	provider.Status.BeginStagingConditions()

	if provider.Type() == api.Ova && provider.DeletionTimestamp == nil {
		if !provider.HasReconciled() {
			// the provider has changed, so delete the old
			// so we can redeploy.
			err = r.DeleteOVAProviderServer(ctx, provider)
			if err != nil {
				return
			}
		}
		err = r.EnsureOVAProviderServer(ctx, provider)
		if err != nil {
			return
		}
	}

	// although the way we deploy OVA servers has changed, we will
	// leave this for now to ensure legacy PVs are cleaned up
	if provider.DeletionTimestamp != nil && k8sutil.ContainsFinalizer(provider, api.OvaProviderFinalizer) {
		err = r.removeVolumeOfOVAServer(provider)
		if err != nil {
			return
		}
		err = r.DeleteOVAProviderServer(ctx, provider)
		if err != nil {
			return
		}
		clonedProvider := provider.DeepCopy()
		k8sutil.RemoveFinalizer(provider, api.OvaProviderFinalizer)
		if pErr := r.Patch(context.TODO(), provider, client.MergeFrom(clonedProvider)); pErr != nil {
			r.Log.Error(pErr, "Failed to remove finalizer", "provider", provider)
			err = pErr
			return
		}
	}

	// Validations.
	err = r.validate(provider)
	if err != nil {
		if err = r.updateProviderStatus(provider); err != nil {
			r.Log.Error(err, "failed to update provider status")
		}
		return
	}

	// Ensure SSH keys for vSphere providers
	if provider.Type() == api.VSphere {
		err = r.ensureSSHKeys(provider)
		if err != nil {
			r.Log.Error(err, "failed to ensure SSH keys for vSphere provider")
			return
		}
	}

	// Update the container.
	err = r.updateContainer(provider)
	if err != nil {
		return
	}

	// Ready condition.
	if !provider.Status.HasBlockerCondition() &&
		provider.Status.HasCondition(ConnectionTestSucceeded, InventoryCreated) {
		provider.Status.Phase = Ready
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

	if err = r.updateProviderStatus(provider); err != nil {
		r.Log.Error(err, "failed to update provider status")
		return
	}

	// Update the DB.
	err = r.updateProvider(provider)
	if err != nil {
		return
	}

	// ReQ.
	if !provider.Status.HasCondition(ConnectionTestSucceeded, InventoryCreated) {
		r.Log.Info(
			"Waiting connection tested or inventory created.")
		result.RequeueAfter = base.SlowReQ
	} else {
		// requeue so that connections are re-tested periodically
		result.RequeueAfter = base.LongReQ
	}

	// Done
	return
}

func (r *Reconciler) updateProviderStatus(provider *api.Provider) error {
	// Record events.
	r.Record(provider, provider.Status.Conditions)

	// Apply changes.
	provider.Status.ObservedGeneration = provider.Generation

	if err := r.Status().Update(context.TODO(), provider); err != nil {
		return err
	}

	return nil
}

// Update the provider.
func (r *Reconciler) updateProvider(provider *api.Provider) (err error) {
	collector, found := r.container.Get(provider)
	if found {
		*(collector.Owner().(*api.Provider)) = *provider
	}

	return
}

// Update the container.
func (r *Reconciler) updateContainer(provider *api.Provider) (err error) {
	if _, found := r.container.Get(provider); found {
		if provider.HasReconciled() {
			r.Log.V(1).Info(
				"Provider not reconciled, postponing.")
			return
		}
	}
	if provider.Status.HasBlockerCondition() ||
		!provider.Status.HasCondition(ConnectionTestSucceeded) {
		r.Log.V(1).Info(
			"Provider not ready, postponing.")
		return
	}
	log.Info("Update container.")
	if current, found := r.container.Get(provider); found {
		current.Shutdown()
		_ = current.DB().Close(true)
		r.Log.V(2).Info(
			"Shutdown found collector.")
	}
	db := r.getDB(provider)
	secret, err := r.getSecret(provider)
	if err != nil {
		return
	}
	err = db.Open(true)
	if err != nil {
		return
	}

	collector := container.Build(db, provider, secret)
	err = r.container.Add(collector)
	if err != nil {
		return
	}

	r.Log.V(2).Info(
		"Data collector added/started.")

	return
}

// Build DB for provider.
func (r *Reconciler) getDB(provider *api.Provider) (db libmodel.DB) {
	dir := Settings.Inventory.WorkingDir
	dir = filepath.Join(
		dir,
		provider.Namespace,
		provider.Name)
	_ = os.MkdirAll(dir, 0755)
	file := string(provider.UID) + ".db"
	path := filepath.Join(dir, file)
	models := model.Models(provider)
	db = libmodel.New(path, models...)
	r.Log.Info(
		"Opening DB.",
		"path",
		path)
	return
}

// Get the secret referenced by the provider.
func (r *Reconciler) getSecret(provider *api.Provider) (*v1.Secret, error) {
	secret := &v1.Secret{}
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

// Provider catalog.
type Catalog struct {
	mutex   sync.Mutex
	content map[reconcile.Request]*api.Provider
}

// Add a provider to the catalog.
func (r *Catalog) add(request reconcile.Request, p *api.Provider) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if r.content == nil {
		r.content = make(map[reconcile.Request]*api.Provider)
	}
	r.content[request] = p
}

// Get a provider from the catalog.
func (r *Catalog) get(request reconcile.Request) (p *api.Provider, found bool) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if r.content == nil {
		r.content = make(map[reconcile.Request]*api.Provider)
	}
	p, found = r.content[request]
	return
}

func (r *Reconciler) removeVolumeOfOVAServer(provider *api.Provider) error {
	labelSelector := labels.SelectorFromSet(labels.Set{
		"subapp":   "ova-server",
		"app":      "forklift",
		"provider": provider.Name,
	})
	pvList := &v1.PersistentVolumeList{}
	if err := r.Client.List(context.TODO(), pvList, &client.ListOptions{LabelSelector: labelSelector}); err != nil {
		r.Log.Error(err, "Failed to list PVs for OVA provider", "provider", provider)
		return err
	} else {
		for _, pv := range pvList.Items {
			if err = r.Client.Delete(context.TODO(), &pv); err != nil {
				r.Log.Error(err, "Failed to delete PV", "PV", pv)
				return err
			}
		}
	}
	return nil
}

// ensureSSHKeys generates and stores SSH keys for vSphere providers if they don't exist
func (r *Reconciler) ensureSSHKeys(provider *api.Provider) error {
	if provider.Type() != api.VSphere {
		return nil
	}

	// Use provider name for SSH key naming
	providerName := provider.Name
	if providerName == "" {
		return fmt.Errorf("provider name is empty")
	}

	// Check if SSH keys already exist
	privateSecretName, err := sshkeys.GenerateSSHPrivateSecretName(providerName)
	if err != nil {
		return fmt.Errorf("failed to generate SSH private secret name: %w", err)
	}
	publicSecretName, err := sshkeys.GenerateSSHPublicSecretName(providerName)
	if err != nil {
		return fmt.Errorf("failed to generate SSH public secret name: %w", err)
	}

	_, err = r.getSSHKeySecret(provider.Namespace, privateSecretName)
	if err == nil {
		// SSH keys already exist, skip generation
		r.Log.V(1).Info("SSH keys already exist for provider", "provider", provider.Name)
		return nil
	}

	if !k8serr.IsNotFound(err) {
		return fmt.Errorf("failed to check for existing SSH private key secret: %w", err)
	}

	// Generate new SSH key pair
	r.Log.Info("Generating SSH keys for vSphere provider", "provider", provider.Name)
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate RSA key: %w", err)
	}

	// Convert private key to PEM format
	privateKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}
	privateKeyBytes := pem.EncodeToMemory(privateKeyPEM)

	// Convert public key to SSH format
	publicKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return fmt.Errorf("failed to create SSH public key: %w", err)
	}
	publicKeyBytes := ssh.MarshalAuthorizedKey(publicKey)

	// Store SSH keys in secrets
	err = r.storeSSHKeySecret(provider.Namespace, privateSecretName, "private-key", privateKeyBytes, provider)
	if err != nil {
		return fmt.Errorf("failed to store private key: %w", err)
	}

	err = r.storeSSHKeySecret(provider.Namespace, publicSecretName, "public-key", publicKeyBytes, provider)
	if err != nil {
		return fmt.Errorf("failed to store public key: %w", err)
	}

	r.Log.Info("SSH keys generated and stored successfully", "provider", provider.Name)
	return nil
}

// getSSHKeySecret retrieves an SSH key secret
func (r *Reconciler) getSSHKeySecret(namespace, secretName string) (*v1.Secret, error) {
	secret := &v1.Secret{}
	key := client.ObjectKey{
		Namespace: namespace,
		Name:      secretName,
	}
	err := r.Get(context.TODO(), key, secret)
	return secret, err
}

// storeSSHKeySecret creates or updates an SSH key secret
func (r *Reconciler) storeSSHKeySecret(namespace, secretName, keyName string, keyData []byte, provider *api.Provider) error {
	providerLabel, err := sshkeys.SanitizeProviderName(provider.Name)
	if err != nil {
		return fmt.Errorf("failed to sanitize provider name for secret label: %w", err)
	}

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":        "forklift",
				"app.kubernetes.io/component":   "ssh-keys",
				"app.kubernetes.io/managed-by":  "forklift-controller",
				"forklift.konveyor.io/provider": providerLabel,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         provider.APIVersion,
					Kind:               provider.Kind,
					Name:               provider.Name,
					UID:                provider.UID,
					Controller:         &[]bool{true}[0],
					BlockOwnerDeletion: &[]bool{true}[0],
				},
			},
		},
		Type: v1.SecretTypeOpaque,
		Data: map[string][]byte{
			keyName: keyData,
		},
	}

	err = r.Create(context.TODO(), secret)
	if err != nil && !k8serr.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create secret %s: %w", secretName, err)
	}

	return nil
}
