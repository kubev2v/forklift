package webhooks

import (
	"net/http"

	"github.com/konveyor/forklift-controller/pkg/lib/logging"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var log = logging.WithName("webhooks")

const ProviderValidatePath = "/provider-validate"
const SecretValidatePath = "/secret-validate"

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

func RegisterValidatingWebhooks() {
	log.Info("register validation webhooks\n")
	mux := http.NewServeMux()
	mux.HandleFunc(ProviderValidatePath, func(w http.ResponseWriter, r *http.Request) {
		ServeProviderCreate(w, r)
	})
	mux.HandleFunc(SecretValidatePath, func(w http.ResponseWriter, r *http.Request) {
		ServeSecretCreate(w, r)
	})
	server := http.Server{
		Addr:    ":8443",
		Handler: mux,
	}
	log.Info("start listening")
	errors := make(chan error)
	go func() {
		errors <- server.ListenAndServeTLS("/var/run/secrets/forklift-api-serving-cert/tls.crt", "/var/run/secrets/forklift-api-serving-cert/tls.key")
	}()
	err := <-errors
	if err != nil {
		log.Info("got error from server")
	}
	log.Info("stopped listening")
}
