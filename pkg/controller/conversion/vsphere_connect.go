package conversion

import (
	"context"
	liburl "net/url"

	"github.com/kubev2v/forklift/pkg/controller/base"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/lib/util"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/soap"
	core "k8s.io/api/core/v1"
)

// GovmomiClientFromSecret builds a logged-in govmomi client using credentials
// and connection parameters stored entirely in secret.Data.
// Required keys: "url", "user", "password".
// Optional keys: "fingerprint" (used when insecureSkipVerify is true).
func GovmomiClientFromSecret(ctx context.Context, secret *core.Secret) (*govmomi.Client, error) {
	if secret == nil {
		return nil, liberr.New("secret is nil")
	}
	urlStr := string(secret.Data["url"])
	if urlStr == "" {
		return nil, liberr.New("connection secret is missing required key 'url'")
	}
	u, err := liburl.Parse(urlStr)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	user := string(secret.Data["user"])
	password := string(secret.Data["password"])
	u.User = liburl.UserPassword(user, password)

	skipVerifying := base.GetInsecureSkipVerifyFlag(secret)
	var thumbprint string
	if skipVerifying {
		thumbprint = string(secret.Data["fingerprint"])
	} else {
		cert, errtls := base.VerifyTLSConnection(urlStr, secret)
		if errtls != nil {
			return nil, liberr.Wrap(errtls)
		}
		thumbprint = util.Fingerprint(cert)
	}

	soapClient := soap.NewClient(u, skipVerifying)
	soapClient.SetThumbprint(u.Host, thumbprint)
	vimClient, err := vim25.NewClient(ctx, soapClient)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	client := &govmomi.Client{
		SessionManager: session.NewManager(vimClient),
		Client:         vimClient,
	}
	err = client.Login(ctx, u.User)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	return client, nil
}
