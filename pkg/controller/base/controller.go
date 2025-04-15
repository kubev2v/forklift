package base

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/url"
	"time"

	"github.com/konveyor/forklift-controller/pkg/controller/provider/container/vsphere"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web"
	libcnd "github.com/konveyor/forklift-controller/pkg/lib/condition"
	"github.com/konveyor/forklift-controller/pkg/lib/logging"
	"github.com/konveyor/forklift-controller/pkg/lib/util"
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

func (r *Reconciler) VerifyTLSConnection(rawURL string, secret *core.Secret) error {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Attempt to get certificate
	cert, err := util.GetTlsCertificate(parsedURL, secret)
	if err != nil {
		if vsphere.GetInsecureSkipVerifyFlag(secret) {
			r.Log.Error(err, "failed to connect to provider", "url", parsedURL)
			return fmt.Errorf("failed to connect to provider: %w", err)
		}
		r.Log.Error(err, "failed to get TLS certificate", "url", parsedURL)
		return fmt.Errorf("failed to get TLS certificate: %w", err)
	}
	if cert == nil {
		return fmt.Errorf("received nil certificate from GetTlsCertificate")
	}

	// Create cert pool
	tlsConfig := &tls.Config{
		RootCAs: x509.NewCertPool(),
	}
	tlsConfig.RootCAs.AddCert(cert)

	// Ensure host:port
	host := parsedURL.Host
	if _, _, err := net.SplitHostPort(parsedURL.Host); err != nil {
		host = parsedURL.Host + ":443"
	}

	// Dial TLS
	conn, err := tls.Dial("tcp", host, tlsConfig)
	if err != nil {
		r.Log.Error(err, "failed to create a secure connection to server")
		return fmt.Errorf("failed to create a secure TLS connection: %w", err)
	}
	defer conn.Close()

	return nil
}
