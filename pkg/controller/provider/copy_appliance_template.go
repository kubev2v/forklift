package provider

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"reflect"
	"slices"
	"time"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/copyappliance"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	"github.com/kubev2v/forklift/pkg/nbd-container/announce"
	v1 "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8sutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// copyApplianceCheckDeadline is how long an appliance has to come up before the check
// gives up on it. Generous: a clone, a boot, a login and a container pull.
const copyApplianceCheckDeadline = 30 * time.Minute

// applianceCheck proves, once per provider, that a copy appliance can be
// deployed from the provider's copy appliance template — the clone, the boot, the
// guest network, the login, the orchestrator install and the export endpoint —
// so that a migration is not the first thing to find out that it cannot.
//
// The verdict is recorded on CopyApplianceChecked and the provider is held
// on CopyApplianceCheckNotReady until that verdict is a pass. Persisting the
// conditions is the caller's Reconcile. Built for a single pass and discarded.
type applianceCheck struct {
	client     client.Client
	appliances copyappliance.Ensurer
	provider   *api.Provider
	// build returns the appliance to deploy. A field because the real one reads
	// the provider's inventory service over the network, which a unit test
	// cannot do.
	build func(*api.Provider, *api.CopyApplianceTemplate) (*api.CopyAppliance, error)
}

func newApplianceCheck(client client.Client, provider *api.Provider) *applianceCheck {
	return &applianceCheck{
		client: client,
		appliances: copyappliance.Ensurer{
			Client: client,
			Log:    logging.WithName("provider|appliance-check"),
		},
		provider: provider,
		build: func(provider *api.Provider, copyApplianceTemplate *api.CopyApplianceTemplate) (*api.CopyAppliance, error) {
			builder, err := copyappliance.NewBuilder(provider)
			if err != nil {
				return nil, liberr.Wrap(err)
			}
			return builder.Check(copyApplianceTemplate)
		},
	}
}

// Run advances the check by one step. Errors are reported to the user as a
// blocking condition before they are returned, so the caller is free to log the
// error rather than propagate it.
func (c *applianceCheck) Run(ctx context.Context) (err error) {
	if c.provider.DeletionTimestamp != nil || !inventoryReady(c.provider) {
		return
	}

	copyApplianceTemplate := &api.CopyApplianceTemplate{}
	key := client.ObjectKey{
		Namespace: c.provider.Namespace,
		Name:      c.provider.CopyApplianceTemplateName(),
	}
	err = c.client.Get(ctx, key, copyApplianceTemplate)
	if k8serr.IsNotFound(err) {
		// The template is created explicitly by the console or the API. Until
		// there is one there is nothing to check and nothing to hold up.
		err = nil
		return
	}
	if err != nil {
		c.block(CopyApplianceCheckPending,
			fmt.Sprintf("Could not read the copy appliance template: %s.", err))
		err = liberr.Wrap(err, "template", key.Name)
		return
	}
	if copyApplianceTemplate.Status.Phase != api.CopyApplianceTemplatePhaseSucceeded ||
		copyApplianceTemplate.Status.Template.Moref == "" {
		c.block(CopyApplianceCheckPending, "Waiting for the copy appliance template.")
		return
	}

	// Items the verdict is keyed on: rebuild the template or change the image
	// and the last verdict says nothing about what is there now.
	items := []string{
		copyApplianceTemplate.Status.Template.Moref,
		copyApplianceTemplate.Status.Template.DiskHash,
		copyApplianceTemplate.Status.Template.ConfigHash,
		Settings.ContainerImage,
	}
	describe := fmt.Sprintf("template %s (disk %s, config %s), appliance image %s",
		items[0], items[1], items[2], items[3])

	appliance, err := c.appliances.Find(ctx,
		c.provider.Namespace,
		c.appliances.Labeler.CheckLabels(c.provider),
		true)
	if err != nil {
		c.block(CopyApplianceCheckPending,
			fmt.Sprintf("Could not read the check appliance: %s.", err))
		return
	}

	// A verdict already reached for these inputs stands, and the appliance it
	// was reached with has served its purpose.
	if recorded := c.provider.Status.FindCondition(CopyApplianceChecked); recorded != nil && slices.Equal(recorded.Items, items) {
		if recorded.Reason != CopyApplianceCheckPassed {
			c.block(CopyApplianceCheckFailed, recorded.Message)
		}
		return c.teardown(ctx, appliance)
	}
	c.provider.Status.DeleteCondition(CopyApplianceChecked)

	switch {
	case appliance == nil:
		err = c.deploy(ctx, copyApplianceTemplate)
	case copyappliance.IsDeployReady(appliance) ||
		// Check appliances have no AttachDisks, so deploy ends at Released.
		appliance.Status.Phase == api.PhaseReleased &&
			appliance.Status.HasCondition(libcnd.Ready):
		c.pass(items, describe)
		err = c.teardown(ctx, appliance)
	case appliance.Status.Phase == api.PhaseDeployFailed:
		c.fail(items, describe, copyappliance.FailureReason(appliance))
		err = c.teardown(ctx, appliance)
	case time.Since(appliance.CreationTimestamp.Time) > copyApplianceCheckDeadline:
		c.fail(items, describe, fmt.Sprintf(
			"the appliance did not come up within %s", copyApplianceCheckDeadline))
		err = c.teardown(ctx, appliance)
	default:
		c.block(CopyApplianceCheckPending,
			fmt.Sprintf("Check running (%s).", appliance.Status.Phase))
	}
	return
}

// deploy creates the check appliance the same way a migration does (via the
// ensurer) and blocks the provider while it comes up.
func (c *applianceCheck) deploy(ctx context.Context, copyApplianceTemplate *api.CopyApplianceTemplate) (err error) {
	defer func() {
		if err != nil {
			c.block(CopyApplianceCheckFailed,
				fmt.Sprintf("Could not deploy the check appliance: %s.", err))
		}
	}()

	appliance, err := c.build(c.provider, copyApplianceTemplate)
	if err != nil {
		return liberr.Wrap(err)
	}
	// Owned by the provider, so deleting the provider takes the appliance with
	// it even if the check never settles.
	if err = k8sutil.SetControllerReference(c.provider, appliance, c.client.Scheme()); err != nil {
		return liberr.Wrap(err)
	}
	if _, err = c.appliances.Appliance(ctx, appliance); err != nil {
		return err
	}
	c.block(CopyApplianceCheckPending, "Copy appliance check started.")
	return
}

// teardown deletes the check appliance, if there is one still to delete. The
// check is a one-off: once the verdict is recorded the appliance has no further
// purpose, and it holds a vCenter VM open until it goes.
func (c *applianceCheck) teardown(ctx context.Context, appliance *api.CopyAppliance) error {
	if appliance == nil || appliance.DeletionTimestamp != nil {
		return nil
	}
	err := c.client.Delete(ctx, appliance)
	if k8serr.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return liberr.Wrap(err, "appliance", appliance.Name)
	}
	return nil
}

// block records that the provider may not be used until the appliance has been
// shown to work. Deliberately not durable: updateContainer and the template
// sync both bail on a blocker and both run before the check, so a blocker that
// survived staging would stop the inventory updating and stop the template
// being rebuilt to fix the very failure it records.
func (c *applianceCheck) block(reason, message string) {
	c.provider.Status.SetCondition(libcnd.Condition{
		Type:     CopyApplianceCheckNotReady,
		Status:   True,
		Reason:   reason,
		Category: Critical,
		Message:  message,
	})
}

// pass records that the check succeeded. Durable so it survives staging;
// Advisory so it never blocks a pass.
func (c *applianceCheck) pass(items []string, describe string) {
	c.provider.Status.SetCondition(libcnd.Condition{
		Type:     CopyApplianceChecked,
		Status:   True,
		Reason:   CopyApplianceCheckPassed,
		Category: Advisory,
		Durable:  true,
		Items:    items,
		Message:  fmt.Sprintf("Copy appliance check passed: %s.", describe),
	})
}

// fail records the failure and blocks on it together so the two cannot drift.
func (c *applianceCheck) fail(items []string, describe, reason string) {
	message := fmt.Sprintf("Copy appliance check failed: %s (%s).", reason, describe)
	c.provider.Status.SetCondition(
		libcnd.Condition{
			Type:     CopyApplianceChecked,
			Status:   True,
			Reason:   CopyApplianceCheckFailed,
			Category: Advisory,
			Durable:  true,
			Items:    items,
			Message:  message,
		},
		libcnd.Condition{
			Type:     CopyApplianceCheckNotReady,
			Status:   True,
			Reason:   CopyApplianceCheckFailed,
			Category: Critical,
			Message:  message,
		})
}

// copyApplianceTemplateSync keeps a provider's copy appliance template in step with the provider's
// settings and the controller's own image settings. The template itself is
// created by the console or the API; this only maintains one that is already
// there. Built for a single reconcile pass and discarded.
type copyApplianceTemplateSync struct {
	client   client.Client
	provider *api.Provider
}

func newCopyApplianceTemplateSync(client client.Client, provider *api.Provider) *copyApplianceTemplateSync {
	return &copyApplianceTemplateSync{client: client, provider: provider}
}

// Run applies the provider's settings to its copy appliance template.
func (s *copyApplianceTemplateSync) Run(ctx context.Context) error {
	if !inventoryReady(s.provider) {
		return nil
	}
	datastore := s.provider.Setting(api.CopyApplianceDatastore)
	folder := s.provider.Setting(api.CopyApplianceFolder)
	network := s.provider.Setting(api.CopyApplianceNetwork)
	if Settings.BaseDiskContainerImage == "" ||
		datastore == "" || folder == "" || network == "" {
		return nil
	}

	template := &api.CopyApplianceTemplate{}
	key := client.ObjectKey{
		Namespace: s.provider.Namespace,
		Name:      s.provider.CopyApplianceTemplateName(),
	}
	err := s.client.Get(ctx, key, template)
	if k8serr.IsNotFound(err) {
		// Created explicitly by the console or the API; not ours to make.
		return nil
	}
	if err != nil {
		return liberr.Wrap(err, "template", key.Name)
	}

	found := template.DeepCopy()
	// Fields the provider dictates. The rest — TransferNetwork and
	// NodeSelector — belong to whoever created the template and are left
	// as found.
	template.Spec.Provider = v1.ObjectReference{
		Name:      s.provider.Name,
		Namespace: s.provider.Namespace,
	}
	template.Spec.TemplateName = s.provider.CopyApplianceTemplateName()
	template.Spec.BaseDisk = api.CopyApplianceTemplateBaseDisk{
		ContainerImage: Settings.BaseDiskContainerImage,
	}
	template.Spec.Resources = api.CopyApplianceTemplateResources{
		CPU:       Settings.TemplateCPU,
		MemoryMiB: Settings.TemplateMemoryMiB,
	}
	template.Spec.Datastore = datastore
	template.Spec.Folder = folder
	template.Spec.Network = network
	template.Spec.BuilderImage = Settings.BuilderImage

	if err = k8sutil.SetControllerReference(s.provider, template, s.client.Scheme()); err != nil {
		return liberr.Wrap(err)
	}
	if reflect.DeepEqual(found.Spec, template.Spec) &&
		reflect.DeepEqual(found.OwnerReferences, template.OwnerReferences) {
		return nil
	}
	// Update rather than Patch: a patch is sent even when the diff is empty,
	// which would turn every pass into a write.
	if err = s.client.Update(ctx, template); err != nil {
		return liberr.Wrap(err, "template", key.Name)
	}
	return nil
}

// Subject names of the issued certificates. The server is named for the logical
// service rather than for an address: the appliance is cloned on demand and its
// address is not known when the certificate is issued, so the client verifies
// the name instead of where it reached it.
const (
	tlsCAName     = "forklift-copy-appliance-template-ca"
	tlsClientName = "forklift-controller"
)

// tlsLifetime is how long the issued certificates are good for. There is no
// renewal: the material is regenerated with the provider's SSH keys, and an
// appliance is a transient clone that outlives neither.
const tlsLifetime = 10 * 365 * 24 * time.Hour

// tlsClockSkew backdates NotBefore so that a verifier whose clock runs behind
// the controller's does not reject a freshly issued certificate.
const tlsClockSkew = time.Hour

// copyApplianceTemplateTLS issues the mutual-TLS material a copyApplianceTemplate appliance serves its
// exports with: a private CA, the server half the appliance presents, and the
// client half the controller queries it with. The CA key is discarded once the
// leaves are signed, so nothing else can be issued against it. Keyed by the
// file name each lands under on the appliance.
func copyApplianceTemplateTLS() (data map[string][]byte, err error) {
	caKey, ca, caPEM, err := issueCertificate(nil, nil, tlsCAName)
	if err != nil {
		return
	}
	serverCertPEM, serverKeyPEM, err := issueLeaf(ca, caKey, announce.ServerName, x509.ExtKeyUsageServerAuth)
	if err != nil {
		return
	}
	clientCertPEM, clientKeyPEM, err := issueLeaf(ca, caKey, tlsClientName, x509.ExtKeyUsageClientAuth)
	if err != nil {
		return
	}
	data = map[string][]byte{
		announce.CACert:     caPEM,
		announce.ServerCert: serverCertPEM,
		announce.ServerKey:  serverKeyPEM,
		announce.ClientCert: clientCertPEM,
		announce.ClientKey:  clientKeyPEM,
	}
	return
}

// ensureCopyApplianceTemplateTLS fills in the TLS material an appliance secret is missing.
// All five are regenerated together: a leaf and a CA from different runs do not
// chain, so replacing only what is absent would leave the secret unusable.
func (r *Reconciler) ensureCopyApplianceTemplateTLS(secret *v1.Secret) error {
	complete := true
	for _, key := range announce.SecretTLSKeys {
		if len(secret.Data[key]) == 0 {
			complete = false
			break
		}
	}
	if complete {
		return nil
	}

	r.Log.Info("Generating copyApplianceTemplate TLS material for existing secret", "secret", secret.Name)
	data, err := copyApplianceTemplateTLS()
	if err != nil {
		return err
	}
	if secret.Data == nil {
		secret.Data = make(map[string][]byte, len(data))
	}
	for key, value := range data {
		secret.Data[key] = value
	}
	err = r.Update(context.TODO(), secret)
	if err != nil {
		return fmt.Errorf("failed to update secret %s with copyApplianceTemplate TLS material: %w", secret.Name, err)
	}
	return nil
}

// issueLeaf signs an end-entity certificate against the CA and returns it with
// its key, both PEM encoded.
func issueLeaf(ca *x509.Certificate, caKey *ecdsa.PrivateKey, name string, eku x509.ExtKeyUsage) (certPEM, keyPEM []byte, err error) {
	key, _, certPEM, err := issueCertificate(ca, caKey, name, eku)
	if err != nil {
		return
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		err = fmt.Errorf("failed to marshal %s key: %w", name, err)
		return
	}
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	return
}

// issueCertificate signs a certificate, self-signing it as a CA when there is
// no parent. A leaf carries the name as a DNS SAN as well as in the subject,
// because that is the half a Go client verifies.
func issueCertificate(parent *x509.Certificate, parentKey *ecdsa.PrivateKey, name string, eku ...x509.ExtKeyUsage) (key *ecdsa.PrivateKey, cert *x509.Certificate, certPEM []byte, err error) {
	key, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		err = fmt.Errorf("failed to generate %s key: %w", name, err)
		return
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		err = fmt.Errorf("failed to generate %s serial: %w", name, err)
		return
	}
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: name},
		NotBefore:             now.Add(-tlsClockSkew),
		NotAfter:              now.Add(tlsLifetime),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           eku,
		BasicConstraintsValid: true,
	}
	if parent == nil {
		template.IsCA = true
		template.KeyUsage |= x509.KeyUsageCertSign
	} else {
		template.DNSNames = []string{name}
	}

	signer, signerKey := template, key
	if parent != nil {
		signer, signerKey = parent, parentKey
	}
	der, err := x509.CreateCertificate(rand.Reader, template, signer, &key.PublicKey, signerKey)
	if err != nil {
		err = fmt.Errorf("failed to sign %s certificate: %w", name, err)
		return
	}
	cert, err = x509.ParseCertificate(der)
	if err != nil {
		err = fmt.Errorf("failed to parse %s certificate: %w", name, err)
		return
	}
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	return
}
