package util

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	liburl "net/url"
	"strconv"
	"time"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/settings"
	core "k8s.io/api/core/v1"
)

func dialTLSWithTimeout(host string, cfg *tls.Config, timeout time.Duration) (*tls.Conn, error) {
	return dialTLSContext(context.Background(), host, cfg, timeout)
}

// dialTLSContext dials a TLS connection with both a per-dial timeout and
// context cancellation on the TCP dial and TLS handshake.
func dialTLSContext(ctx context.Context, host string, cfg *tls.Config, timeout time.Duration) (*tls.Conn, error) {
	dialer := &net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", host)
	if err != nil {
		return nil, err
	}
	tlsConn := tls.Client(conn, cfg)
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return tlsConn, nil
}

func extractServerName(address string) string {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		// If SplitHostPort fails (likely no port is present), assume the entire address is the host.
		return address
	}
	return host
}

// GetTlsCertificate retrieves the server's TLS certificate.
// Compatibility wrapper; use GetTlsCertificateContext for cancellation support.
func GetTlsCertificate(url *liburl.URL, secret *core.Secret) (*x509.Certificate, error) {
	return GetTlsCertificateContext(context.Background(), url, secret)
}

// GetTlsCertificateContext retrieves the server's TLS certificate,
// respecting the supplied context for cancellation on both the TCP
// dial and the TLS handshake.
func GetTlsCertificateContext(ctx context.Context, url *liburl.URL, secret *core.Secret) (crt *x509.Certificate, err error) {
	cfg, err := tlsConfig(secret)
	if err != nil {
		return
	}
	if url.Host == "" {
		url.Host = url.Path
	}
	host := url.Host
	if host == "" {
		err = liberr.New("URL host or path is empty")
		return
	}
	if url.Port() == "" {
		host += ":443"
	}
	cfg.ServerName = extractServerName(host)
	cfg.InsecureSkipVerify = true
	conn, err := dialTLSContext(ctx, host, cfg, time.Duration(settings.Settings.TlsConnectionTimeout)*time.Second)
	if err != nil {
		err = liberr.Wrap(err, "url", url)
		return
	}
	defer conn.Close()
	if len(conn.ConnectionState().PeerCertificates) > 0 {
		crt = conn.ConnectionState().PeerCertificates[0]
	} else {
		err = liberr.Wrap(fmt.Errorf("no peer certificates"), "url", url)
	}
	return
}

func tlsConfig(secret *core.Secret) (cfg *tls.Config, err error) {
	cfg = &tls.Config{}
	if InsecureProvider(secret) {
		cfg.InsecureSkipVerify = true
	} else if cacert, ok := GetCACert(secret); ok {
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

// GetCACert retrieves the CA certificate from a secret.
// It first checks for the standard Kubernetes field "ca.crt",
// then falls back to the legacy "cacert" field for backward compatibility.
// Returns the certificate bytes and a boolean indicating if found.
func GetCACert(secret *core.Secret) ([]byte, bool) {
	// Try standard Kubernetes field first
	if cacrt, ok := secret.Data["ca.crt"]; ok && len(cacrt) > 0 {
		return cacrt, true
	}
	// The following field is deprecated and will be removed in Forklift v2.14.
	if cacert, ok := secret.Data["cacert"]; ok && len(cacert) > 0 {
		return cacert, true
	}
	return nil, false
}

func HasCACert(secret *core.Secret) bool {
	_, ok := GetCACert(secret)
	return ok
}
