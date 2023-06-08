package utils

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	"net"
	"net/http"
	"time"
)

func TestHttpsCA(url string, caCert string, isInsecure bool) (err error) {
	var TLSClientConfig *tls.Config

	TLSClientConfig = &tls.Config{InsecureSkipVerify: isInsecure}

	cacert := []byte(caCert)
	roots := x509.NewCertPool()
	ok := roots.AppendCertsFromPEM(cacert)
	if !ok {
		fmt.Sprintf("the CA certificate is malformed or was not provided, falling back to system CA cert pool")
		roots, err = x509.SystemCertPool()
		if err != nil {
			err = liberr.New("failed to configure the system's cert pool")
			return
		}
	}
	TLSClientConfig = &tls.Config{RootCAs: roots}

	HTTPClient := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 10 * time.Second,
			}).DialContext,
			MaxIdleConns:          10,
			IdleConnTimeout:       10 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			TLSClientConfig:       TLSClientConfig,
		},
	}
	_, err = HTTPClient.Get(url)
	return
}
