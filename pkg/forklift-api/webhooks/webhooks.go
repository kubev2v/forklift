package webhooks

import (
	"net/http"

	"github.com/konveyor/forklift-controller/pkg/lib/logging"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var log = logging.WithName("webhooks")

const SecretValidatePath = "/secret-validate"
const SecretMutatorPath = "/secret-mutate"

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

}

func RegisterMutatingWebhooks(mux *http.ServeMux) {
	log.Info("register mutation webhook")
	mux.HandleFunc(SecretMutatorPath, func(w http.ResponseWriter, r *http.Request) {
		ServeSecretMutator(w, r)
	})
}
