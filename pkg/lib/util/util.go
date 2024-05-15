package util

import (
	"bytes"
	"crypto/sha1"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	liburl "net/url"
	"strconv"

	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	core "k8s.io/api/core/v1"
)

func GetTlsCertificate(url *liburl.URL, secret *core.Secret) (crt *x509.Certificate, err error) {
	cfg, err := tlsConfig(secret)
	if err != nil {
		return
	}

	host := url.Host
	if url.Port() == "" {
		host += ":443"
	}

	conn, err := tls.Dial("tcp", host, cfg)
	if err == nil && len(conn.ConnectionState().PeerCertificates) > 0 {
		crt = conn.ConnectionState().PeerCertificates[0]
	} else {
		err = liberr.Wrap(err, "url", url)
	}
	return
}

func tlsConfig(secret *core.Secret) (cfg *tls.Config, err error) {
	cfg = &tls.Config{}
	if InsecureProvider(secret) {
		cfg.InsecureSkipVerify = true
	} else if cacert, ok := secret.Data["cacert"]; ok {
		cfg.RootCAs = x509.NewCertPool()
		if ok := cfg.RootCAs.AppendCertsFromPEM(cacert); !ok {
			err = liberr.New("failed to parse the specified certificate")
		}
	} else {
		if cfg.RootCAs, err = x509.SystemCertPool(); err != nil {
			err = liberr.Wrap(err)
		}
	}
	return
}

func Fingerprint(cert *x509.Certificate) string {
	sum := sha1.Sum(cert.Raw)
	var buf bytes.Buffer
	for i, f := range sum {
		if i > 0 {
			fmt.Fprintf(&buf, ":")
		}
		fmt.Fprintf(&buf, "%02X", f)
	}
	return buf.String()
}

func InsecureProvider(secret *core.Secret) bool {
	insecure, found := secret.Data[api.Insecure]
	if !found {
		return false
	}

	insecureSkipVerify, err := strconv.ParseBool(string(insecure))
	if err != nil {
		return false
	}

	return insecureSkipVerify
}
