package util

import (
	"bytes"
	"crypto/sha1"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	liburl "net/url"
	"strconv"
	"strings"
	"time"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/settings"
	core "k8s.io/api/core/v1"
)

func dialTLSWithTimeout(host string, cfg *tls.Config, timeout time.Duration) (*tls.Conn, error) {
	conn, err := net.DialTimeout("tcp", host, timeout)
	if err != nil {
		return nil, err
	}

	tlsConn := tls.Client(conn, cfg)

	// Set handshake timeout
	err = tlsConn.Handshake()
	if err != nil {
		conn.Close()
		return nil, err
	}

	return tlsConn, nil
}

func extractServerName(address string) string {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		// If SplitHostPort fails (likely no port is present), fallback
		return strings.Split(address, ":")[0]
	}
	return host
}

func GetTlsCertificate(url *liburl.URL, secret *core.Secret) (crt *x509.Certificate, err error) {
	cfg, err := tlsConfig(secret)
	if err != nil {
		return
	}
	host := ""
	if url.Host == "" {
		//There are cases where the URL is provided without a host, e.g. "https://path/to/resource"
		url.Host = url.Path
	}
	host = url.Host

	if host == "" {
		err = liberr.New("URL host or path is empty")
		return
	}
	if url.Port() == "" {
		host += ":443"
	}

	//cfg.ServerName ensures the TLS handshake checks the correct hostname is in the server’s certificate
	cfg.ServerName = extractServerName(host)
	// disable verification since we don't trust it yet
	cfg.InsecureSkipVerify = true
	conn, err := dialTLSWithTimeout(host, cfg, time.Duration(settings.Settings.TlsConnectionTimeout)*time.Second)
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
