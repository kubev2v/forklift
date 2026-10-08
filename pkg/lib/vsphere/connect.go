package vsphere

import (
	"context"
	liburl "net/url"
	"strconv"

	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/lib/util"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/soap"
	core "k8s.io/api/core/v1"
)

// Connect logs into a vSphere endpoint.
func Connect(ctx context.Context, rawURL, user, password, thumbprint string, insecure bool) (*govmomi.Client, error) {
	url, err := soap.ParseURL(rawURL)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	url.User = liburl.UserPassword(user, password)
	soapClient := soap.NewClient(url, insecure)
	if thumbprint != "" {
		soapClient.SetThumbprint(url.Host, thumbprint)
	}
	vimClient, err := vim25.NewClient(ctx, soapClient)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	client := &govmomi.Client{
		SessionManager: session.NewManager(vimClient),
		Client:         vimClient,
	}
	if err = client.Login(ctx, url.User); err != nil {
		return nil, liberr.Wrap(err)
	}
	return client, nil
}

// ConnectProvider applies provider-secret TLS semantics then Connect.
func ConnectProvider(ctx context.Context, rawURL, user, password, thumbprint string, secret *core.Secret) (*govmomi.Client, error) {
	if secret == nil {
		return nil, liberr.New("secret is nil")
	}
	insecure := InsecureFromSecret(secret)
	if !insecure {
		u, err := liburl.Parse(rawURL)
		if err != nil {
			return nil, liberr.Wrap(err)
		}
		cert, err := util.GetTlsCertificate(u, secret)
		if err != nil {
			return nil, liberr.Wrap(err)
		}
		if cert == nil {
			return nil, liberr.New("received nil certificate while verifying TLS")
		}
		thumbprint = util.Fingerprint(cert)
	}
	return Connect(ctx, rawURL, user, password, thumbprint, insecure)
}

// ConnectFromSecret builds a client from secret keys url/user/password.
func ConnectFromSecret(ctx context.Context, secret *core.Secret) (*govmomi.Client, error) {
	if secret == nil {
		return nil, liberr.New("secret is nil")
	}
	rawURL := string(secret.Data["url"])
	user := string(secret.Data["user"])
	password := string(secret.Data["password"])
	if rawURL == "" || user == "" || password == "" {
		return nil, liberr.New("connection secret missing url, user, or password")
	}
	thumbprint := ""
	if InsecureFromSecret(secret) {
		thumbprint = string(secret.Data["fingerprint"])
	}
	return ConnectProvider(ctx, rawURL, user, password, thumbprint, secret)
}

// InsecureFromSecret reads insecureSkipVerify from a provider secret.
func InsecureFromSecret(secret *core.Secret) bool {
	if secret == nil {
		return false
	}
	v, err := strconv.ParseBool(string(secret.Data["insecureSkipVerify"]))
	return err == nil && v
}
