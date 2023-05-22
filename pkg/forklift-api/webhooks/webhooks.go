package webhooks

import (
	"net/http"

	"github.com/konveyor/forklift-controller/pkg/lib/logging"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var log = logging.WithName("webhooks")

const SecretValidatePath = "/secret-validate"
const SecretMutatorPath = "/secret-mutate"
const PlanValidatePath = "/plan-validate"
const PlanMutatorPath = "/plan-mutate"
const ProviderValidatePath = "/provider-validate"

// AddToManagerFuncs is a list of functions to add all Controllers to the Manager
var AddToManagerFuncs []func(manager.Manager) error

// AddToManager adds all Controllers to the Manager
func AddToManager(m manager.Manager) error {
	for _, f := range AddToManagerFuncs {
		if err := f(m); err != nil {
			return err
		}
	}
	return nil
}

func RegisterValidatingWebhooks(mux *http.ServeMux) {
	log.Info("register validation webhooks")
	mux.HandleFunc(SecretValidatePath, func(w http.ResponseWriter, r *http.Request) {
		ServeSecretCreate(w, r)
	})
	mux.HandleFunc(PlanValidatePath, func(w http.ResponseWriter, r *http.Request) {
		ServePlanCreate(w, r)
	})
	mux.HandleFunc(ProviderValidatePath, func(w http.ResponseWriter, r *http.Request) {
		ServeProviderCreate(w, r)
	})
}

func RegisterMutatingWebhooks(mux *http.ServeMux) {
	log.Info("register mutation webhook")
	mux.HandleFunc(SecretMutatorPath, func(w http.ResponseWriter, r *http.Request) {
		ServeSecretMutator(w, r)
	})
	mux.HandleFunc(PlanMutatorPath, func(w http.ResponseWriter, r *http.Request) {
		ServePlanMutator(w, r)
	})
}
