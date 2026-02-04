package services

import (
	"net/http"

	"github.com/kubev2v/forklift/pkg/lib/logging"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const TLS_CERTIFICATE_PATH = "/tls-certificate"

var log = logging.WithName("services")

func RegisterServices(mux *http.ServeMux, k8sClient client.Client) {
	log.Info("register TLS certificate service")
	mux.HandleFunc(TLS_CERTIFICATE_PATH, func(w http.ResponseWriter, r *http.Request) {
		serveTlsCertificate(w, r, k8sClient)
	})
	log.Info("register hooks-from-ansible service")
	mux.HandleFunc(HooksFromAnsiblePath, func(w http.ResponseWriter, r *http.Request) {
		serveHooksFromAnsible(w, r, k8sClient)
	})
}
