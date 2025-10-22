package webhooks

import (
	"net/http"

	mutating_webhooks "github.com/kubev2v/forklift/pkg/forklift-api/webhooks/mutating-webhook"
	"github.com/kubev2v/forklift/pkg/forklift-api/webhooks/mutating-webhook/mutators"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ServeSecretMutator(resp http.ResponseWriter, req *http.Request, client client.Client) {
	mutating_webhooks.Serve(resp, req, &mutators.SecretMutator{Client: client})
}

func ServePlanMutator(resp http.ResponseWriter, req *http.Request, client client.Client) {
	mutating_webhooks.Serve(resp, req, &mutators.PlanMutator{Client: client})
}

func ServeProviderMutator(resp http.ResponseWriter, req *http.Request, client client.Client) {
	mutating_webhooks.Serve(resp, req, &mutators.ProviderMutator{Client: client})
}
