package webhooks

import (
	"net/http"

	validating_webhooks "github.com/konveyor/forklift-controller/pkg/forklift-api/webhooks/validating-webhook"
	"github.com/konveyor/forklift-controller/pkg/forklift-api/webhooks/validating-webhook/admitters"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

func ServeMigrationCreate(resp http.ResponseWriter, req *http.Request, client client.Client) {
	validating_webhooks.Serve(resp, req, &admitters.MigrationAdmitter{Client: client})
}
