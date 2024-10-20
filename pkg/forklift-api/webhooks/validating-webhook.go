package webhooks

import (
	"net/http"

	"sigs.k8s.io/controller-runtime/pkg/client"

	validating_webhooks "github.com/konveyor/forklift-controller/pkg/forklift-api/webhooks/validating-webhook"
	"github.com/konveyor/forklift-controller/pkg/forklift-api/webhooks/validating-webhook/admitters"
)

func ServeSecretCreate(resp http.ResponseWriter, req *http.Request, client client.Client) {
	validating_webhooks.Serve(resp, req, &admitters.SecretAdmitter{Client: client})
}

func ServePlanCreate(resp http.ResponseWriter, req *http.Request, client client.Client) {
	validating_webhooks.Serve(resp, req, &admitters.PlanAdmitter{Client: client})
}

func ServeProviderCreate(resp http.ResponseWriter, req *http.Request, client client.Client) {
	validating_webhooks.Serve(resp, req, &admitters.ProviderAdmitter{Client: client})
}
