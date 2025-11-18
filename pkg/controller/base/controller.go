package base

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"time"

	"github.com/kubev2v/forklift/pkg/controller/provider/web"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	"github.com/kubev2v/forklift/pkg/lib/util"
	core "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	FastReQ = time.Millisecond * 500
	SlowReQ = time.Second * 3
	LongReQ = time.Second * 30
)

// Base reconciler.
type Reconciler struct {
	record.EventRecorder
	client.Client
	Log logging.LevelLogger
}

// Reconcile started.
func (r *Reconciler) Started() {
	r.Log.Info("Reconcile started.")
}

// Reconcile ended.
func (r *Reconciler) Ended(reQin time.Duration, err error) (reQ time.Duration) {
	defer func() {
		r.Log.Info(
			"Reconcile ended.",
			"reQ",
			reQ)
	}()
	reQ = reQin
	if err == nil {
		return
	}
	reQ = SlowReQ
	if k8serr.IsConflict(err) {
		r.Log.Info(err.Error())
		return
	}
	if errors.As(err, &web.ProviderNotReadyError{}) {
		r.Log.V(1).Info(
			"Provider inventory not ready.")
		return
	}
	r.Log.Error(
		err,
		"Reconcile failed.")

	return
}

// Record for changes in conditions.
// Logged and recorded as `Event`.
func (r *Reconciler) Record(object runtime.Object, cnd libcnd.Conditions) {
	explain := cnd.Explain()
	record := func(cnd libcnd.Condition) {
		event := ""
		switch cnd.Category {
		case libcnd.Critical,
			libcnd.Error,
			libcnd.Warn:
			event = core.EventTypeWarning
		default:
			event = core.EventTypeNormal
		}
		r.EventRecorder.Event(
			object,
			event,
			cnd.Type,
			cnd.Message)
	}
	for _, cnd := range explain.Added {
		r.Log.Info(
			"Condition added.",
			"condition",
			cnd)
		record(cnd)
	}
	for _, cnd := range explain.Updated {
		r.Log.Info(
			"Condition updated.",
			"condition",
			cnd)
		record(cnd)
	}
	for _, cnd := range explain.Deleted {
		r.Log.Info(
			"Condition deleted.",
			"condition",
			cnd)
		record(cnd)
	}
}

// GetInsecureSkipVerifyFlag gets the insecureSkipVerify boolean flag
// value from the provider connection secret.
func GetInsecureSkipVerifyFlag(secret *core.Secret) bool {
	insecure, found := secret.Data["insecureSkipVerify"]
	if !found {
		return false
	}

	insecureSkipVerify, err := strconv.ParseBool(string(insecure))
	if err != nil {
		return false
	}

	return insecureSkipVerify
}

func VerifyTLSConnection(rawURL string, secret *core.Secret) (*x509.Certificate, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Get the TLS certificate from the server
	cert, err := util.GetTlsCertificate(parsedURL, secret)
	if err != nil {
		return nil, fmt.Errorf("failed to get TLS certificate: %w", err)
	}
	if cert == nil {
		return nil, fmt.Errorf("received nil certificate from GetTlsCertificate")
	}

	// Prepare TLS config with the retrieved certificate
	tlsConfig := &tls.Config{
		RootCAs: x509.NewCertPool(),
	}
	tlsConfig.RootCAs.AddCert(cert)

	// Extract hostname and port (add :443 if no port specified)
	hostname, port, err := net.SplitHostPort(parsedURL.Host)
	if err != nil {
		hostname = parsedURL.Host
		port = "443"
	}

	// Check if hostname is actually an IP address
	isHostnameIP := net.ParseIP(hostname) != nil

	// Determine if certificate matches the hostname directly
	certMatchesHostname := false
	if !isHostnameIP {
		// Hostname (not IP): check if it exists in certificate's DNS names
		for _, dnsName := range cert.DNSNames {
			if dnsName == hostname {
				certMatchesHostname = true
				break
			}
		}
		// Fallback: check if Common Name (CN) matches
		if !certMatchesHostname && cert.Subject.CommonName == hostname {
			certMatchesHostname = true
		}
	}

	// If hostname doesn't match certificate, attempt IP resolution and matching
	connectionHostname := hostname
	if !certMatchesHostname && !isHostnameIP {
		// Resolve hostname to IP addresses
		if ips, err := net.LookupIP(hostname); err == nil && len(ips) > 0 {
			// Check each resolved IP against certificate's IP addresses
			for _, resolvedIP := range ips {
				for _, certIP := range cert.IPAddresses {
					if certIP.Equal(resolvedIP) {
						// Found matching IP - use for connection
						connectionHostname = resolvedIP.String()
						break
					}
				}
				if connectionHostname != hostname {
					break
				}
			}
		}
	}

	// Set ServerName only if connecting with a hostname (not IP)
	if net.ParseIP(connectionHostname) == nil {
		tlsConfig.ServerName = connectionHostname
	}

	// Establish TLS connection with hostname or matched IP
	conn, err := tls.Dial("tcp", net.JoinHostPort(connectionHostname, port), tlsConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create a secure TLS connection: %w", err)
	}
	defer conn.Close()

	return cert, nil
}
