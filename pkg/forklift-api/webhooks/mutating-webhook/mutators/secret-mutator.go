package mutators

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/forklift-api/webhooks/util"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	libutil "github.com/kubev2v/forklift/pkg/lib/util"
	admissionv1 "k8s.io/api/admission/v1beta1"
	core "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var log = logging.WithName("mutator")

const NewLine = 0x0a

type SecretMutator struct {
	secret core.Secret
	Client client.Client
}

var resourceTypeToMutateFunc = map[string]func(*SecretMutator) *admissionv1.AdmissionResponse{
	"providers": func(mutator *SecretMutator) *admissionv1.AdmissionResponse {
		return mutator.mutateProviderSecret()
	},
	"hosts": func(mutator *SecretMutator) *admissionv1.AdmissionResponse {
		return mutator.mutateHostSecret()
	},
}

func (mutator *SecretMutator) Mutate(ar *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	log.Info("secret mutator was called")
	raw := ar.Request.Object.Raw
	if err := json.Unmarshal(raw, &mutator.secret); err != nil {
		log.Error(err, "mutating webhook error, failed to unmarshel certificate")
		return util.ToAdmissionResponseError(err)
	}

	// A no-op rewrite (e.g. by the storage version migrator or an encryption key
	// rotation) must not trigger external calls or denial (MTV-7041). Data and
	// Labels are the only fields this mutator ever changes.
	if ar.Request.Operation == admissionv1.Update && mutator.isUnchanged(ar.Request.OldObject.Raw) {
		log.Info("secret unchanged, skipping mutation")
		return util.ToAdmissionResponseAllow()
	}

	// The label createdForResourceType must exist due to the configuration of the webhook
	resourceType := mutator.secret.GetLabels()["createdForResourceType"]
	if mutate, ok := resourceTypeToMutateFunc[resourceType]; ok {
		return mutate(mutator)
	}

	return util.ToAdmissionResponseAllow()
}

// isUnchanged reports whether the updated secret differs from the stored one
// in any field this mutator may modify.
func (mutator *SecretMutator) isUnchanged(oldRaw []byte) bool {
	if len(oldRaw) == 0 {
		return false
	}
	old := &core.Secret{}
	if err := json.Unmarshal(oldRaw, old); err != nil {
		return false
	}
	return apiequality.Semantic.DeepEqual(mutator.secret.Data, old.Data) &&
		apiequality.Semantic.DeepEqual(mutator.secret.Labels, old.Labels)
}

func (mutator *SecretMutator) mutateProviderSecret() *admissionv1.AdmissionResponse {
	var insecure, secretChanged bool
	if insecureSkipVerify, ok := mutator.secret.Data["insecureSkipVerify"]; ok {
		var err error
		if insecure, err = strconv.ParseBool(string(insecureSkipVerify)); err != nil {
			log.Error(err, "Failed to parse insecure property from the secret")
			return util.ToAdmissionResponseError(err)
		}
	} else {
		mutator.secret.Data["insecureSkipVerify"] = []byte("false")
		secretChanged = true
	}

	if providerType := mutator.secret.GetLabels()["createdForProviderType"]; providerType == "ovirt" && !insecure {
		url, err := url.Parse(string(mutator.secret.Data["url"]))
		if err != nil {
			log.Error(err, "mutating webhook URL parsing error")
			return util.ToAdmissionResponseError(err)
		}

		certPool := x509.NewCertPool()
		existingCACert, hasCert := libutil.GetCACert(&mutator.secret)
		if !hasCert {
			err = liberr.Wrap(errors.New("CA certificate not found in secret"))
			log.Error(err, "CA certificate is missing")
			return &admissionv1.AdmissionResponse{
				Allowed: false,
				Result: &metav1.Status{
					Message: "CA certificate is required for oVirt provider in secure mode (insecureSkipVerify=false).",
					Code:    http.StatusBadRequest,
				},
			}
		}
		ok := certPool.AppendCertsFromPEM(existingCACert)
		if !ok {
			err = liberr.Wrap(errors.New("failed to parse certificate"))
			log.Error(err, "Certificate is not valid")
			return &admissionv1.AdmissionResponse{
				Allowed: false,
				Result: &metav1.Status{
					Message: "The certificate is not valid, can't proceed.",
					Code:    http.StatusBadRequest,
				},
			}
		}
		certUrl := fmt.Sprint(url.Scheme, "://", url.Host, "/ovirt-engine/services/pki-resource?resource=ca-certificate&format=X509-PEM-CA")
		// Retrieving the engine CA must not block the write: the webhook has
		// failurePolicy Fail, and denying rewrites of the secret (e.g. by the
		// encryption migration controller) hangs cluster operators (MTV-7041).
		// Connection problems are surfaced by the provider controller instead.
		cert, err := fetchEngineCACertificate(certUrl, certPool)
		if err != nil {
			log.Error(err, "failed to retrieve the engine CA certificate, allowing the secret without updating it")
		} else if !contains(existingCACert, cert) {
			// Normalize and migrate the CA cert to the new format.
			mutator.secret.Data["ca.crt"] = appendCerts(existingCACert, cert)

			// Optional: clean up the legacy field while we're here to accelerate migration.
			delete(mutator.secret.Data, "cacert")

			mutator.secret.Labels["ca-cert-updated"] = "true"
			secretChanged = true
			log.Info("Engine CA certificate was missing, updating the secret")
		}
	}

	if secretChanged {
		return mutator.patchSecret()
	}

	// Response for other providers type or insecure mode
	return &admissionv1.AdmissionResponse{
		Allowed: true,
		Result: &metav1.Status{
			Message: "Certificate retrieval is not required, passing ",
			Code:    http.StatusOK,
		},
	}
}

func (mutator *SecretMutator) patchSecret() *admissionv1.AdmissionResponse {
	patchBytes, err := util.GeneratePatchPayload(
		util.PatchOperation{
			Op:    "replace",
			Path:  "/data",
			Value: mutator.secret.Data,
		},
		util.PatchOperation{
			Op:    "replace",
			Path:  "/metadata/labels",
			Value: mutator.secret.Labels,
		},
	)

	if err != nil {
		log.Error(err, "mutating webhook error, failed to generete paylod for patch request")
		return util.ToAdmissionResponseError(err)
	}

	jsonPatchType := admissionv1.PatchTypeJSONPatch
	return &admissionv1.AdmissionResponse{
		Allowed:   true,
		Patch:     patchBytes,
		PatchType: &jsonPatchType,
	}
}

func (mutator *SecretMutator) mutateHostSecret() *admissionv1.AdmissionResponse {
	if _, ok := mutator.secret.GetLabels()["createdForResource"]; ok { // checking this just because there's no point in mutating an invalid secret
		var secretChanged bool
		if user, ok := mutator.secret.Data["user"]; !ok || string(user) == "" {
			// Fetch the provider secret if the user is not set
			provider := &api.Provider{}
			providerName := string(mutator.secret.Data["provider"])
			providerNamespace := mutator.secret.Namespace
			if err := mutator.Client.Get(context.TODO(), client.ObjectKey{Namespace: providerNamespace, Name: providerName}, provider); err != nil {
				log.Error(err, "failed to find provider for Host secret without credentials")
				return util.ToAdmissionResponseError(err)
			}
			if provider.Spec.Settings[api.SDK] == api.ESXI {
				ref := provider.Spec.Secret
				providerSecret := &core.Secret{}
				if err := mutator.Client.Get(context.TODO(), client.ObjectKey{Namespace: ref.Namespace, Name: ref.Name}, providerSecret); err != nil {
					log.Error(err, "failed to get provider secret for Host secret without credentials")
					return util.ToAdmissionResponseError(err)
				}
				mutator.secret.Data["user"] = providerSecret.Data["user"]
				mutator.secret.Data["password"] = providerSecret.Data["password"]
				secretChanged = true
				log.Info("copied credentials from ESXi provider to its Host")
			}
		}

		if _, hasInsecureSkipVerify := mutator.secret.Data["insecureSkipVerify"]; !hasInsecureSkipVerify {
			// Fetch the provider to get the insecureSkipVerify setting
			provider := &api.Provider{}
			providerName := string(mutator.secret.Data["provider"])
			providerNamespace := mutator.secret.Namespace
			if err := mutator.Client.Get(context.TODO(), client.ObjectKey{Namespace: providerNamespace, Name: providerName}, provider); err != nil {
				log.Error(err, "failed to find provider for insecureSkipVerify setting")
				return util.ToAdmissionResponseError(err)
			}

			// Fetch the provider secret
			ref := provider.Spec.Secret
			providerSecret := &core.Secret{}
			if err := mutator.Client.Get(context.TODO(), client.ObjectKey{Namespace: ref.Namespace, Name: ref.Name}, providerSecret); err != nil {
				log.Error(err, "failed to get provider secret for insecureSkipVerify setting")
				return util.ToAdmissionResponseError(err)
			}

			// Copy the skip verify setting from the provider secret if it exists
			if insecureSkipVerify, ok := providerSecret.Data["insecureSkipVerify"]; ok {
				mutator.secret.Data["insecureSkipVerify"] = insecureSkipVerify
				secretChanged = true
				log.Info("copied insecureSkipVerify setting from provider secret to host secret")
			}
		}

		if secretChanged {
			return mutator.patchSecret()
		}
	}
	return util.ToAdmissionResponseAllow()
}

// fetchEngineCACertificate retrieves the oVirt engine CA certificate, verifying
// the engine against the CA provided in the secret. The timeout must stay well
// below the webhook's timeoutSeconds (30).
func fetchEngineCACertificate(certUrl string, certPool *x509.CertPool) ([]byte, error) {
	client := &http.Client{
		// The transport is per-request, so a kept-alive connection would linger
		// idle after the fetch returns with nobody to reuse or close it.
		Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: certPool}, DisableKeepAlives: true},
		Timeout:   10 * time.Second,
	}
	response, err := client.Get(certUrl)
	if err != nil {
		return nil, err
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("engine returned status %s when retrieving its CA certificate", response.Status)
	}
	return io.ReadAll(response.Body)
}

func contains(secretCert, cert []byte) bool {
	flatSecretCa := bytes.ReplaceAll(secretCert, []byte{NewLine}, []byte{})
	flatCert := bytes.ReplaceAll(cert, []byte{NewLine}, []byte{})
	return bytes.Contains(flatSecretCa, flatCert)
}

func appendCerts(secretCert, cert []byte) []byte {
	if !bytes.HasSuffix(secretCert, []byte{NewLine}) {
		secretCert = append(secretCert, NewLine)
	}
	return append(secretCert, cert...)
}
