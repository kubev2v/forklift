package webhooks

import (
	"net/http"

	mutating_webhooks "github.com/konveyor/forklift-controller/pkg/forklift-api/webhooks/mutating-webhook"
	"github.com/konveyor/forklift-controller/pkg/forklift-api/webhooks/mutating-webhook/mutators"
)

func ServeSecretMutator(resp http.ResponseWriter, req *http.Request) {
	mutating_webhooks.Serve(resp, req, &mutators.SecretMutator{})
}
