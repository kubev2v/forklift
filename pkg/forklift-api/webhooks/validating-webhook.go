package webhooks

import (
	"net/http"

	validating_webhooks "github.com/konveyor/forklift-controller/pkg/forklift-api/webhooks/validating-webhook"
	"github.com/konveyor/forklift-controller/pkg/forklift-api/webhooks/validating-webhook/admitters"
)

func ServeProviderCreate(resp http.ResponseWriter, req *http.Request) {
	validating_webhooks.Serve(resp, req, &admitters.ProviderAdmitter{})
}

func ServeSecretCreate(resp http.ResponseWriter, req *http.Request) {
	validating_webhooks.Serve(resp, req, &admitters.SecretAdmitter{})
}
