package mutators

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/konveyor/forklift-controller/pkg/forklift-api/webhooks/util"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	"github.com/konveyor/forklift-controller/pkg/lib/logging"
	admissionv1 "k8s.io/api/admission/v1beta1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var log = logging.WithName("mutator")

const NewLine = 0x0a

type SecretMutator struct {
}

func (mutator *SecretMutator) Mutate(ar *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	log.Info("secret mutator was called")
	raw := ar.Request.Object.Raw
	secret := &core.Secret{}
	err := json.Unmarshal(raw, secret)
	if err != nil {
		log.Error(err, "mutating webhook error, failed to unmarshel certificate")
		return util.ToAdmissionResponseError(err)
	}

	var insecure, secretChanged bool
	// Applies to all secrets with 'createdForProviderType' label.
	if insecureSkipVerify, ok := secret.Data["insecureSkipVerify"]; ok {
		insecure, err = strconv.ParseBool(string(insecureSkipVerify))
		if err != nil {
			log.Error(err, "Failed to parse insecure property from the secret")
			return util.ToAdmissionResponseError(err)
		}
	} else {
		secret.Data["insecureSkipVerify"] = []byte("false")
		secretChanged = true
	}

	if providerType := secret.GetLabels()["createdForProviderType"]; providerType == "ovirt" && !insecure {
		url, err := url.Parse(string(secret.Data["url"]))
		if err != nil {
			log.Error(err, "mutating webhook URL parsing error")
			return util.ToAdmissionResponseError(err)
		}

		certPool := x509.NewCertPool()
		ok := certPool.AppendCertsFromPEM(secret.Data["cacert"])
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

		cert, err := io.ReadAll(response.Body)
		if err != nil {
			log.Error(err, "mutating webhook error, failed to read certificate retrieval response")
			return util.ToAdmissionResponseError(err)
		}

		//check if the CA included in the secrete provided by the user and update it if needed
		if !contains(secret.Data["cacert"], cert) {
			secret.Data["cacert"] = appendCerts(secret.Data["cacert"], cert)
			secret.Labels["ca-cert-updated"] = "true"
			secretChanged = true
			log.Info("Engine CA certificate was missing, updating the secret")
		}
	}

	//In case the data in the secret changed patch the secret.
	if secretChanged {
		patchBytes, err := util.GeneratePatchPayload(
			util.PatchOperation{
				Op:    "replace",
				Path:  "/data",
				Value: secret.Data,
			},
			util.PatchOperation{
				Op:    "replace",
				Path:  "/metadata/labels",
				Value: secret.Labels,
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

	// Response for other providers type or insecure mode
	return &admissionv1.AdmissionResponse{
		Allowed: true,
		Result: &metav1.Status{
			Message: "Certificate retrieval is not required, passing ",
			Code:    http.StatusOK,
		},
	}
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
