package services

import (
	"net/http"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/konveyor/forklift-controller/pkg/lib/logging"
)

const TLS_CERTIFICATE_PATH = "/tls-certificate"

var log = logging.WithName("services")

func RegisterServices(mux *http.ServeMux, client client.Client) {
	log.Info("register TLS certificate service")
	mux.HandleFunc(TLS_CERTIFICATE_PATH, func(w http.ResponseWriter, r *http.Request) {
		serveTlsCertificate(w, r, client)
	})
}
