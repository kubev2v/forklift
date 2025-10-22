package services

import (
	"encoding/pem"
	"fmt"
	"net/http"
	"net/url"

	"github.com/kubev2v/forklift/pkg/lib/util"
	core "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func serveTlsCertificate(resp http.ResponseWriter, req *http.Request, client client.Client) {
	if url, err := url.Parse(req.URL.Query().Get("URL")); err == nil {
		log.Info("received a request to retrieve certificate", "URL", url)
		secret := &core.Secret{
			Data: map[string][]byte{"insecureSkipVerify": []byte("true")},
		}
		if cacert, err := util.GetTlsCertificate(url, secret); err == nil {
			encoded := pem.EncodeToMemory(&pem.Block{
				Type:  "CERTIFICATE",
				Bytes: cacert.Raw,
			})
			if _, err := resp.Write(encoded); err == nil {
				resp.Header().Set("Content-Type", "text/plain")
				resp.WriteHeader(http.StatusOK)
			} else {
				msg := fmt.Sprintf("failed to write certificate: %s", string(encoded))
				http.Error(resp, msg, http.StatusInternalServerError)
			}
		} else {
			http.Error(resp, err.Error(), http.StatusInternalServerError)
		}
	} else {
		log.Error(err, "received invalid URL", "URL", req.URL.Query().Get("URL"))
		http.Error(resp, "Required parameter is invalid: URL", http.StatusBadRequest)
	}
}
