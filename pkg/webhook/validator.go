package webhook

import (
	"context"
	"fmt"
	"net/http"

	liberr "github.com/konveyor/controller/pkg/error"
	"github.com/konveyor/controller/pkg/logging"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/container"
	core "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	// Name.
	Name = "admission-webhook"
)

// Package logger.
var log = logging.WithName(Name)

type Handler struct {
	Client  client.Client
	decoder *admission.Decoder
}

//
// Build custom provider with parameters form secret
func buildProvider(provider *api.Provider, providerType *api.ProviderType, secret *core.Secret) {
	provider.Spec.URL = string(secret.Data["url"])
	provider.Spec.Type = providerType
	provider.Name = secret.Name
	provider.Namespace = secret.Namespace
}

//
// Admission webhook handler
// Tests the connection to the provider before creating the secret
func (a *Handler) Handle(ctx context.Context, req admission.Request) admission.Response {
	secret := &core.Secret{}
	err := a.decoder.Decode(req, secret)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	// Get the provider type on which start the tests
	createdForProviderType, ok := secret.GetLabels()["createdForProviderType"]
	if !ok {
		return admission.Errored(http.StatusBadRequest, liberr.New("The label 'createdForProviderType' is not set on secret"))
	}
	provider := &api.Provider{}
	providerType := api.ProviderType(createdForProviderType)
	buildProvider(provider, &providerType, secret)

	collector := container.Build(nil, provider, secret)
	if collector == nil {
		return admission.Errored(http.StatusBadRequest, liberr.New(fmt.Sprintf("Incorrect 'createdForProviderType' value. Options %s", api.ProviderTypes)))
	}
	log.Info("Starting provider connection test")
	err = collector.Test()
	if err != nil {
		return admission.ValidationResponse(false, err.Error())
	}
	log.Info("Provider connection test passed")
	return admission.ValidationResponse(true, "Passed the validaiton")
}

// Injects the decoder of the API request
func (p *Handler) InjectDecoder(d *admission.Decoder) error {
	p.decoder = d
	return nil
}
