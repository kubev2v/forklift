package announce

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/kubev2v/forklift/pkg/nbd-container/runner"
)

// ServerName is the logical name the announce server's certificate is issued
// for. A server is brought up on demand at an address nobody knows when the
// certificate is signed, so the certificate names this instead and the client
// verifies against it rather than against wherever it reached the server.
const ServerName = "nbd-server"

// clientTimeout bounds one query. The endpoint answers from memory, so anything
// slower than this is a host that is not really there.
const clientTimeout = 10 * time.Second

// Client queries an announce endpoint over mutual TLS.
type Client struct {
	http *http.Client
}

// NewClient builds a client from the client half of the TLS material: the CA
// every certificate is signed by, and the certificate and key this client
// presents. All three are PEM.
func NewClient(ca, certificate, key []byte) (*Client, error) {
	pair, err := tls.X509KeyPair(certificate, key)
	if err != nil {
		return nil, fmt.Errorf("loading client keypair: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(ca) {
		return nil, fmt.Errorf("the CA bundle contained no valid certificates")
	}

	return &Client{
		http: &http.Client{
			Timeout: clientTimeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					MinVersion:   tls.VersionTLS12,
					RootCAs:      pool,
					Certificates: []tls.Certificate{pair},
					// The server is reached by address but named
					// ServerName; see the constant.
					ServerName: ServerName,
				},
			},
		},
	}, nil
}

// Disks returns the exports the announce server at addr (host:port) publishes.
func (r *Client) Disks(ctx context.Context, addr string) ([]runner.Export, error) {
	url := "https://" + addr + "/disks"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	response, err := r.http.Do(request)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = response.Body.Close()
	}()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, response.Status)
	}

	var exports []runner.Export
	if err := json.NewDecoder(response.Body).Decode(&exports); err != nil {
		return nil, fmt.Errorf("decoding the export list from %s: %w", url, err)
	}
	return exports, nil
}
