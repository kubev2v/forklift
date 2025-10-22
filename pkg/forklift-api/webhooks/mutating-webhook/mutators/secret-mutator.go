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

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/forklift-api/webhooks/util"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	admissionv1 "k8s.io/api/admission/v1beta1"
	core "k8s.io/api/core/v1"
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

	// The label createdForResourceType must exist due to the configuration of the webhook
	resourceType := mutator.secret.GetLabels()["createdForResourceType"]
	if mutate, ok := resourceTypeToMutateFunc[resourceType]; ok {
		return mutate(mutator)
	}

	return util.ToAdmissionResponseAllow()
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
		ok := certPool.AppendCertsFromPEM(mutator.secret.Data["cacert"])
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
		http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{RootCAs: certPool}
		response, err := http.Get(certUrl)
		if err != nil {
			log.Error(err, "mutating webhook error, failed to send request for CA certificate retrieval")
			return util.ToAdmissionResponseError(err)
		}
		defer response.Body.Close()

		cert, err := io.ReadAll(response.Body)
		if err != nil {
			log.Error(err, "mutating webhook error, failed to read certificate retrieval response")
			return util.ToAdmissionResponseError(err)
		}

		//check if the CA included in the secrete provided by the user and update it if needed
		if !contains(mutator.secret.Data["cacert"], cert) {
			mutator.secret.Data["cacert"] = appendCerts(mutator.secret.Data["cacert"], cert)
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
			if secretChanged {
				return mutator.patchSecret()
			}
		}
	}
	return util.ToAdmissionResponseAllow()
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
