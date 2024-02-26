package metrics

import (
	"fmt"
	"net/http"
	"os"
	"path"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/client-go/util/cert"
	"k8s.io/klog/v2"
)

func StartPrometheusEndpoint(host, port, certsDirectory string) {
	certBytes, keyBytes, err := cert.GenerateSelfSignedCertKey(host, nil, nil)
	if err != nil {
		klog.Error("Error generating cert for prometheus")
		return
	}

	certFile := path.Join(certsDirectory, "tls.crt")
	if err = os.WriteFile(certFile, certBytes, 0600); err != nil {
		klog.Error("Error writing cert file")
		return
	}

	keyFile := path.Join(certsDirectory, "tls.key")
	if err = os.WriteFile(keyFile, keyBytes, 0600); err != nil {
		klog.Error("Error writing key file")
		return
	}

	go func() {
		http.Handle("/metrics", promhttp.Handler())
		if err := http.ListenAndServeTLS(fmt.Sprintf(":%s", port), certFile, keyFile, nil); err != nil {
			klog.Warning("Error starting prometheus endpoint: ", err)
			return
		}
	}()
}
