package webhooks

import (
	"net/http"

	"github.com/konveyor/forklift-controller/pkg/lib/logging"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var log = logging.WithName("webhooks")

const SecretValidatePath = "/secret-validate"
const SecretMutatorPath = "/secret-mutate"
const PlanValidatePath = "/plan-validate"
const PlanMutatorPath = "/plan-mutate"
const ProviderValidatePath = "/provider-validate"
const ProviderMutatorPath = "/provider-mutate"
const MigrationValidatePath = "/migration-validate"

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

func RegisterValidatingWebhooks(mux *http.ServeMux, client client.Client) {
	log.Info("register validation webhooks")
	mux.HandleFunc(SecretValidatePath, func(w http.ResponseWriter, r *http.Request) {
		ServeSecretCreate(w, r, client)
	})
	mux.HandleFunc(PlanValidatePath, func(w http.ResponseWriter, r *http.Request) {
		ServePlanCreate(w, r, client)
	})
	mux.HandleFunc(ProviderValidatePath, func(w http.ResponseWriter, r *http.Request) {
		ServeProviderCreate(w, r, client)
	})
	mux.HandleFunc(MigrationValidatePath, func(w http.ResponseWriter, r *http.Request) {
		ServeMigrationCreate(w, r, client)
	})
}

func RegisterMutatingWebhooks(mux *http.ServeMux, client client.Client) {
	log.Info("register mutation webhook")
	mux.HandleFunc(SecretMutatorPath, func(w http.ResponseWriter, r *http.Request) {
		ServeSecretMutator(w, r, client)
	})
	mux.HandleFunc(PlanMutatorPath, func(w http.ResponseWriter, r *http.Request) {
		ServePlanMutator(w, r, client)
	})
	mux.HandleFunc(ProviderMutatorPath, func(w http.ResponseWriter, r *http.Request) {
		ServeProviderMutator(w, r, client)
	})
}
