package admitters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/konveyor/forklift-controller/pkg/apis"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	adapter "github.com/konveyor/forklift-controller/pkg/controller/plan/adapter/vsphere"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/container"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/vsphere"
	webhookutils "github.com/konveyor/forklift-controller/pkg/forklift-api/webhooks/util"
	libcontainer "github.com/konveyor/forklift-controller/pkg/lib/inventory/container"
	"github.com/konveyor/forklift-controller/pkg/lib/logging"
	"github.com/konveyor/forklift-controller/pkg/settings"
	admissionv1 "k8s.io/api/admission/v1beta1"
	core "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Application settings.
var Settings = &settings.Settings

var log = logging.WithName("admitter")

func init() {
	err := Settings.Inventory.Load()
	if err != nil {
		panic(err)
	}
}

type SecretAdmitter struct {
	secret core.Secret
}

func (admitter *SecretAdmitter) Admit(ar *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	log.Info("secret admitter was called")
	raw := ar.Request.Object.Raw

	err := json.Unmarshal(raw, &admitter.secret)
	if err != nil {
		log.Error(err, "secret webhook error, failed to unmarshel secret")
		return webhookutils.ToAdmissionResponseError(err)
	}

	if createdForProviderType, ok := admitter.secret.GetLabels()["createdForProviderType"]; ok {
		providerType := api.ProviderType(createdForProviderType)
		collector, err := admitter.buildProviderCollector(&providerType)
		if err != nil {
			return webhookutils.ToAdmissionResponseError(err)
		}

		log.Info("Starting provider connection test")
		status, err := collector.Test()
		switch {
		case err != nil && (status == http.StatusUnauthorized || status == http.StatusBadRequest):
			log.Info("Connection test failed, failing", "status", status)
			return &admissionv1.AdmissionResponse{
				Allowed: false,
				Result: &metav1.Status{
					Code:    http.StatusForbidden,
					Message: "Invalid credentials",
				},
			}
		case err != nil:
			log.Info("Connection test failed, yet passing", "status", status, "error", err.Error())
		default:
			log.Info("Test succeeded, passing")
		}
		return webhookutils.ToAdmissionResponseAllow()
	}

	if resourceType, ok := admitter.secret.GetLabels()["createdForResourceType"]; ok {
		switch resourceType {
		case "hosts":
			if hostName, ok := admitter.secret.GetLabels()["createdForResource"]; ok {
				if tested, err := admitter.validateHostSecret(hostName); err != nil {
					if tested {
						return &admissionv1.AdmissionResponse{
							Allowed: false,
							Result: &metav1.Status{
								Code:    http.StatusForbidden,
								Message: err.Error(),
							},
						}
					} else {
						return webhookutils.ToAdmissionResponseError(err)
					}
				}
			} else {
				err = errors.New("host secret is labeled with 'createdForResourceType' but without 'createdForResource'")
				return webhookutils.ToAdmissionResponseError(err)
			}
		}
	}

	return webhookutils.ToAdmissionResponseAllow()
}

func (admitter *SecretAdmitter) buildProviderCollector(providerType *api.ProviderType) (libcontainer.Collector, error) {
	provider := &api.Provider{
		ObjectMeta: metav1.ObjectMeta{
			Name:      admitter.secret.Name,
			Namespace: admitter.secret.Namespace,
		},
		Spec: api.ProviderSpec{
			Type: providerType,
			URL:  string(admitter.secret.Data["url"]),
		},
	}

	if collector := container.Build(nil, provider, &admitter.secret); collector != nil {
		return collector, nil
	} else {
		return nil, fmt.Errorf("incorrect 'createdForProviderType' value. Options %s", api.ProviderTypes)
	}
}

func (admitter *SecretAdmitter) validateHostSecret(hostName string) (tested bool, err error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Error(err, "Couldn't get the cluster configuration", err.Error())
		return
	}

	err = api.SchemeBuilder.AddToScheme(scheme.Scheme)
	if err != nil {
		log.Error(err, "Couldn't build the scheme", err.Error())
		return
	}
	err = apis.AddToScheme(scheme.Scheme)
	if err != nil {
		log.Error(err, "Couldn't add forklift API to the scheme", err.Error())
		return
	}

	cl, err := client.New(config, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		log.Error(err, "Couldn't create a cluster client", err.Error())
		return
	}

	provider := &api.Provider{}
	providerName := string(admitter.secret.Data["provider"])
	// there is an assumption that the provider resides within the same namespace as the secret
	// which is reasonable as the hosts are also created on the same namespace as the provider
	// but anyway, if that's not the case, we would likely pass the validation (due to IsNotFound check)
	providerNamespace := admitter.secret.Namespace
	err = cl.Get(context.TODO(), client.ObjectKey{Namespace: providerNamespace, Name: providerName}, provider)
	if err != nil {
		if k8serr.IsNotFound(err) {
			log.Info("Failed to find provider of host, passing")
			err = nil
			return
		} else {
			log.Error(err, "Couldn't get the target provider", err.Error())
			return
		}
	}

	log.Info("Testing provider connection")
	tested, err = admitter.testConnectionToHost(provider, hostName)
	if !tested && err != nil {
		log.Error(err, "Couldn't test connection to the host")
		err = fmt.Errorf("failed to initiate test connection to the host. Error: %s", err.Error())
	}
	return tested, err
}

func (admitter *SecretAdmitter) testConnectionToHost(provider *api.Provider, host string) (bool, error) {
	switch provider.Type() {
	case api.VSphere:
		inventory, err := web.NewClient(provider)
		if err != nil {
			return false, err
		}
		hostModel := &vsphere.Host{}
		err = inventory.Get(hostModel, host)
		if err != nil {
			return false, err
		}
		admitter.secret.Data["thumbprint"] = []byte(hostModel.Thumbprint)
		url := fmt.Sprintf("https://%s/sdk", admitter.secret.Data["ip"])
		h := adapter.EsxHost{
			Secret: &admitter.secret,
			URL:    url,
		}
		if err = h.TestConnection(); err != nil {
			log.Info("Test connection to the host failed")
			err = fmt.Errorf("could not connect to the ESXi host, please check credentials. Error: %s", err.Error())
		} else {
			log.Info("Connection test, succeeded")
		}
		return true, err
	default:
		return true, nil
	}
}
