package webhooks

import (
	"net/http"

	validating_webhooks "github.com/konveyor/forklift-controller/pkg/forklift-api/webhooks/validating-webhook"
	"github.com/konveyor/forklift-controller/pkg/forklift-api/webhooks/validating-webhook/admitters"
)

func ServeSecretCreate(resp http.ResponseWriter, req *http.Request) {
	validating_webhooks.Serve(resp, req, &admitters.SecretAdmitter{})
}
